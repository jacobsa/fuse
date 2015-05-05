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
	"fmt"
	"log"
	"path"
	"runtime"
	"sync"

	"golang.org/x/net/context"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/fuse/fuseops"
)

// A connection to the fuse kernel process.
type Connection struct {
	logger      *log.Logger
	wrapped     *bazilfuse.Conn
	opsInFlight sync.WaitGroup

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
}

// Responsibility for closing the wrapped connection is transferred to the
// result. You must call c.close() eventually.
func newConnection(
	parentCtx context.Context,
	logger *log.Logger,
	wrapped *bazilfuse.Conn) (c *Connection, err error) {
	c = &Connection{
		logger:      logger,
		wrapped:     wrapped,
		parentCtx:   parentCtx,
		cancelFuncs: make(map[bazilfuse.RequestID]func()),
	}

	return
}

// Log information for an operation with the given ID. calldepth is the depth
// to use when recovering file:line information with runtime.Caller.
func (c *Connection) log(
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
	c.logger.Println(msg)
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

// Set up state for an op that is about to be returned to the user, given its
// underlying bazilfuse request.
//
// Return a context that should be used for the op.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) beginOp(
	bfReq bazilfuse.Request) (ctx context.Context) {
	reqID := bfReq.Hdr().ID

	// Note that the op is in flight.
	c.opsInFlight.Add(1)

	// Set up a cancellation function.
	//
	// Special case: On Darwin, osxfuse appears to aggressively reuse "unique"
	// request IDs. This matters for Forget requests, which have no reply
	// associated and therefore appear to have IDs that are immediately eligible
	// for reuse. For these, we should not record any state keyed on their ID.
	//
	// Cf. https://github.com/osxfuse/osxfuse/issues/208
	ctx = c.parentCtx
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

	// Decrement the in-flight counter.
	c.opsInFlight.Done()
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
		c.log(opID, 1, "<- %v", bfReq)

		// Special case: responding to this is required to make mounting work on OS
		// X. We don't currently expose the capability for the file system to
		// intercept this.
		if statfsReq, ok := bfReq.(*bazilfuse.StatfsRequest); ok {
			c.log(opID, 1, "-> (Statfs) OK")
			statfsReq.Respond(&bazilfuse.StatfsResponse{})
			continue
		}

		// Set up op dependencies.
		opCtx := c.beginOp(bfReq)

		logForOp := func(calldepth int, format string, v ...interface{}) {
			c.log(opID, calldepth+1, format, v...)
		}

		finished := func(err error) { c.finishOp(bfReq) }

		op = fuseops.Convert(opCtx, bfReq, logForOp, finished)
		return
	}
}

func (c *Connection) waitForReady() (err error) {
	<-c.wrapped.Ready
	err = c.wrapped.MountError
	return
}

// Close the connection and wait for in-flight ops.
func (c *Connection) close() (err error) {
	err = c.wrapped.Close()
	c.opsInFlight.Wait()
	return
}
