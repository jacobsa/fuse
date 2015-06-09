// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fuse

import (
	"flag"
	"fmt"
	"log"
	"path"
	"runtime"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/reqtrace"
)

var fTraceByPID = flag.Bool(
	"fuse.trace_by_pid",
	false,
	"Enable a hacky mode that uses reqtrace to group all ops from each "+
		"individual PID. Not a good idea to use in production; races, bugs, and "+
		"resource leaks likely lurk.")

// A connection to the fuse kernel process.
type Connection struct {
	debugLogger *log.Logger
	errorLogger *log.Logger
	wrapped     *bazilfuse.Conn

	// The context from which all op contexts inherit.
	parentCtx context.Context

	// For logging purposes only.
	nextOpID uint32

	mu sync.Mutex

	// A map from bazilfuse request ID (*not* the op ID for logging used above)
	// to a function that cancel's its associated context.
	//
	// GUARDED_BY(mu)
	cancelFuncs map[bazilfuse.RequestID]func()

	// A map from PID to a traced context for that PID.
	//
	// GUARDED_BY(mu)
	pidMap map[int]context.Context
}

// Responsibility for closing the wrapped connection is transferred to the
// result. You must call c.close() eventually.
func newConnection(
	parentCtx context.Context,
	debugLogger *log.Logger,
	errorLogger *log.Logger,
	wrapped *bazilfuse.Conn) (c *Connection, err error) {
	c = &Connection{
		debugLogger: debugLogger,
		errorLogger: errorLogger,
		wrapped:     wrapped,
		parentCtx:   parentCtx,
		cancelFuncs: make(map[bazilfuse.RequestID]func()),
		pidMap:      make(map[int]context.Context),
	}

	return
}

// Log information for an operation with the given ID. calldepth is the depth
// to use when recovering file:line information with runtime.Caller.
func (c *Connection) debugLog(
	opID uint32,
	calldepth int,
	format string,
	v ...interface{}) {
	// Get file:line info.
	var file string
	var line int
	var ok bool

	_, file, line, ok = runtime.Caller(calldepth)
	if !ok {
		file = "???"
	}

	fileLine := fmt.Sprintf("%v:%v", path.Base(file), line)

	// Format the actual message to be printed.
	msg := fmt.Sprintf(
		"Op 0x%08x %24s] %v",
		opID,
		fileLine,
		fmt.Sprintf(format, v...))

	// Print it.
	c.debugLogger.Println(msg)
}

// LOCKS_EXCLUDED(c.mu)
func (c *Connection) recordCancelFunc(
	reqID bazilfuse.RequestID,
	f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.cancelFuncs[reqID]; ok {
		panic(fmt.Sprintf("Already have cancel func for request %v", reqID))
	}

	c.cancelFuncs[reqID] = f
}

// Wait until the process completes, then close off the trace and remove the
// context from the map.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) reportWhenPIDGone(
	pid int,
	ctx context.Context,
	report reqtrace.ReportFunc) {
	// HACK(jacobsa): Poll until the process no longer exists.
	const pollPeriod = 50 * time.Millisecond
	for {
		// The man page for kill(2) says that if the signal is zero, then "no
		// signal is sent, but error checking is still performed; this can be used
		// to check for the existence of a process ID".
		err := unix.Kill(pid, 0)

		// ESRCH means the process is gone.
		if err == unix.ESRCH {
			break
		}

		// If we receive EPERM, we're not going to be able to do what we want. We
		// don't really have any choice but to print info and leak.
		if err == unix.EPERM {
			log.Printf("Failed to kill(2) PID %v; no permissions. Leaking trace.", pid)
			return
		}

		// Otherwise, panic.
		if err != nil {
			panic(fmt.Errorf("Kill(%v): %v", pid, err))
		}

		time.Sleep(pollPeriod)
	}

	// Finish up.
	report(nil)

	c.mu.Lock()
	delete(c.pidMap, pid)
	c.mu.Unlock()
}

// Set up a hacky per-PID trace context, if enabled. Either way, return a
// context from which an operation should inherit.
//
// See notes on fTraceByPID.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) maybeTraceByPID(
	pid int) (ctx context.Context) {
	ctx = c.parentCtx

	// Is there anything to do?
	if !reqtrace.Enabled() || !*fTraceByPID {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Do we already have a traced context for this PID?
	if existing, ok := c.pidMap[pid]; ok {
		ctx = existing
		return
	}

	// Set up a new one and stick it in the map.
	var report reqtrace.ReportFunc
	ctx, report = reqtrace.Trace(ctx, fmt.Sprintf("Requests from PID %v", pid))
	c.pidMap[pid] = ctx

	// Ensure we close the trace and remove it from the map eventually.
	go c.reportWhenPIDGone(pid, ctx, report)

	return
}

// Set up state for an op that is about to be returned to the user, given its
// underlying bazilfuse request.
//
// Return a context that should be used for the op.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) beginOp(
	bfReq bazilfuse.Request) (ctx context.Context) {
	reqID := bfReq.Hdr().ID

	// Choose a parent context.
	ctx = c.maybeTraceByPID(int(bfReq.Hdr().Pid))

	// Set up a cancellation function.
	//
	// Special case: On Darwin, osxfuse aggressively reuses "unique" request IDs.
	// This matters for Forget requests, which have no reply associated and
	// therefore have IDs that are immediately eligible for reuse. For these, we
	// should not record any state keyed on their ID.
	//
	// Cf. https://github.com/osxfuse/osxfuse/issues/208
	if _, ok := bfReq.(*bazilfuse.ForgetRequest); !ok {
		var cancel func()
		ctx, cancel = context.WithCancel(ctx)
		c.recordCancelFunc(reqID, cancel)
	}

	return
}

// Clean up all state associated with an op to which the user has responded,
// given its underlying bazilfuse request. This must be called before a
// response is sent to the kernel, to avoid a race where the request's ID might
// be reused by osxfuse.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) finishOp(bfReq bazilfuse.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := bfReq.Hdr().ID

	// Even though the op is finished, context.WithCancel requires us to arrange
	// for the cancellation function to be invoked. We also must remove it from
	// our map.
	//
	// Special case: we don't do this for Forget requests. See the note in
	// beginOp above.
	if _, ok := bfReq.(*bazilfuse.ForgetRequest); !ok {
		cancel, ok := c.cancelFuncs[reqID]
		if !ok {
			panic(fmt.Sprintf("Unknown request ID in finishOp: %v", reqID))
		}

		cancel()
		delete(c.cancelFuncs, reqID)
	}
}

// LOCKS_EXCLUDED(c.mu)
func (c *Connection) handleInterrupt(req *bazilfuse.InterruptRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// NOTE(jacobsa): fuse.txt in the Linux kernel documentation
	// (https://goo.gl/H55Dnr) defines the kernel <-> userspace protocol for
	// interrupts.
	//
	// In particular, my reading of it is that an interrupt request cannot be
	// delivered to userspace before the original request. The part about the
	// race and EAGAIN appears to be aimed at userspace programs that
	// concurrently process requests (cf. http://goo.gl/BES2rs).
	//
	// So in this method if we can't find the ID to be interrupted, it means that
	// the request has already been replied to.
	//
	// Cf. https://github.com/osxfuse/osxfuse/issues/208
	// Cf. http://comments.gmane.org/gmane.comp.file-systems.fuse.devel/14675
	cancel, ok := c.cancelFuncs[req.IntrID]
	if !ok {
		return
	}

	cancel()
}

// Read the next op from the kernel process. Return io.EOF if the kernel has
// closed the connection.
//
// This function delivers ops in exactly the order they are received from
// /dev/fuse. It must not be called multiple times concurrently.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) ReadOp() (op fuseops.Op, err error) {
	// Keep going until we find a request we know how to convert.
	for {
		// Read a bazilfuse request.
		var bfReq bazilfuse.Request
		bfReq, err = c.wrapped.ReadRequest()

		if err != nil {
			return
		}

		// Choose an ID for this operation.
		opID := c.nextOpID
		c.nextOpID++

		// Log the receipt of the operation.
		c.debugLog(opID, 1, "<- %v", bfReq)

		// Special case: responding to statfs is required to make mounting work on
		// OS X. We don't currently expose the capability for the file system to
		// intercept this.
		if statfsReq, ok := bfReq.(*bazilfuse.StatfsRequest); ok {
			c.debugLog(opID, 1, "-> (Statfs) OK")
			statfsReq.Respond(&bazilfuse.StatfsResponse{})
			continue
		}

		// Special case: handle interrupt requests.
		if interruptReq, ok := bfReq.(*bazilfuse.InterruptRequest); ok {
			c.handleInterrupt(interruptReq)
			continue
		}

		// Set up op dependencies.
		opCtx := c.beginOp(bfReq)

		debugLogForOp := func(calldepth int, format string, v ...interface{}) {
			c.debugLog(opID, calldepth+1, format, v...)
		}

		finished := func(err error) { c.finishOp(bfReq) }

		op = fuseops.Convert(
			opCtx,
			bfReq,
			debugLogForOp,
			c.errorLogger,
			finished)

		return
	}
}

func (c *Connection) waitForReady() (err error) {
	<-c.wrapped.Ready
	err = c.wrapped.MountError
	return
}

// Close the connection. Must not be called until operations that were read
// from the connection have been responded to.
func (c *Connection) close() (err error) {
	err = c.wrapped.Close()
	return
}
