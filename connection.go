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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"sync"
	"syscall"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/internal/buffer"
	"github.com/jacobsa/fuse/internal/freelist"
	"github.com/jacobsa/fuse/internal/fusekernel"
)

type contextKeyType uint64

var contextKey interface{} = contextKeyType(0)

// Ask the Linux kernel for larger read requests.
//
// As of 2015-03-26, the behavior in the kernel is:
//
//   - (https://tinyurl.com/2eakn5e9, https://tinyurl.com/mry9e33d) Set the
//     local variable ra_pages to be init_response->max_readahead divided by
//     the page size.
//
//   - (https://tinyurl.com/2eakn5e9, https://tinyurl.com/mbpshk8h) Set
//     backing_dev_info::ra_pages to the min of that value and what was sent in
//     the request's max_readahead field.
//
//   - (https://tinyurl.com/57hpfu4x) Use backing_dev_info::ra_pages when
//     deciding how much to read ahead.
//
//   - (https://tinyurl.com/ywhfcfte) Don't read ahead at all if that field is
//     zero.
//
// Reading a page at a time is a drag. Ask for a larger size.
const maxReadahead = 1 << 20

// Connection represents a connection to the fuse kernel process. It is used to
// receive and reply to requests from the kernel.
type Connection struct {
	cfg         MountConfig
	debugLogger *log.Logger
	errorLogger *log.Logger

	// The device through which we're talking to the kernel, and the protocol
	// version that we're using to talk to it.
	dev      *os.File
	protocol fusekernel.Protocol

	mu sync.Mutex

	// A map from fuse "unique" request ID (*not* the op ID for logging used
	// above) to a function that cancel's its associated context.
	//
	// GUARDED_BY(mu)
	cancelFuncs map[uint64]func()

	// Freelists, serviced by freelists.go.
	inMessages  freelist.Freelist // GUARDED_BY(mu)
	outMessages freelist.Freelist // GUARDED_BY(mu)
}

// State that is maintained for each in-flight op. This is stuffed into the
// context that the user uses to reply to the op.
type opState struct {
	inMsg  *buffer.InMessage
	outMsg *buffer.OutMessage
	op     interface{}
}

// Create a connection wrapping the supplied file descriptor connected to the
// kernel. You must eventually call c.close().
//
// The loggers may be nil.
func newConnection(
	cfg MountConfig,
	debugLogger *log.Logger,
	errorLogger *log.Logger,
	dev *os.File) (*Connection, error) {
	c := &Connection{
		cfg:         cfg,
		debugLogger: debugLogger,
		errorLogger: errorLogger,
		dev:         dev,
		cancelFuncs: make(map[uint64]func()),
	}

	// Initialize.
	if err := c.Init(); err != nil {
		c.close()
		return nil, fmt.Errorf("Init: %v", err)
	}

	return c, nil
}

// Init performs the work necessary to cause the mount process to complete.
func (c *Connection) Init() error {
	// Read the init op.
	ctx, op, err := c.ReadOp()
	if err != nil {
		return fmt.Errorf("Reading init op: %v", err)
	}

	initOp, ok := op.(*initOp)
	if !ok {
		c.Reply(ctx, syscall.EPROTO)
		return fmt.Errorf("Expected *initOp, got %T", op)
	}

	// Make sure the protocol version spoken by the kernel is new enough.
	min := fusekernel.Protocol{
		fusekernel.ProtoVersionMinMajor,
		fusekernel.ProtoVersionMinMinor,
	}

	if initOp.Kernel.LT(min) {
		c.Reply(ctx, syscall.EPROTO)
		return fmt.Errorf("Version too old: %v", initOp.Kernel)
	}

	// Downgrade our protocol if necessary.
	c.protocol = fusekernel.Protocol{
		fusekernel.ProtoVersionMaxMajor,
		fusekernel.ProtoVersionMaxMinor,
	}

	if initOp.Kernel.LT(c.protocol) {
		c.protocol = initOp.Kernel
	}

	cacheSymlinks := initOp.Flags&fusekernel.InitCacheSymlinks > 0
	noOpenSupport := initOp.Flags&fusekernel.InitNoOpenSupport > 0
	noOpendirSupport := initOp.Flags&fusekernel.InitNoOpendirSupport > 0

	// Respond to the init op.
	initOp.Library = c.protocol
	initOp.MaxReadahead = maxReadahead
	initOp.MaxWrite = buffer.MaxWriteSize

	initOp.Flags = 0

	// Tell the kernel not to use pitifully small 4 KiB writes.
	initOp.Flags |= fusekernel.InitBigWrites

	if c.cfg.EnableAsyncReads {
		initOp.Flags |= fusekernel.InitAsyncRead
	}

	// kernel 4.20 increases the max from 32 -> 256
	initOp.Flags |= fusekernel.InitMaxPages
	initOp.MaxPages = 256

	// Enable writeback caching if the user hasn't asked us not to.
	if !c.cfg.DisableWritebackCaching {
		initOp.Flags |= fusekernel.InitWritebackCache
	}

	// Enable caching symlink targets in the kernel page cache if the user opted
	// into it (might require fixing the size field of inode attributes first):
	if c.cfg.EnableSymlinkCaching && cacheSymlinks {
		initOp.Flags |= fusekernel.InitCacheSymlinks
	}

	// Tell the kernel to treat returning -ENOSYS on OpenFile as not needing
	// OpenFile calls at all (Linux >= 3.16):
	if c.cfg.EnableNoOpenSupport && noOpenSupport {
		initOp.Flags |= fusekernel.InitNoOpenSupport
	}

	// Tell the kernel to treat returning -ENOSYS on OpenDir as not needing
	// OpenDir calls at all (Linux >= 5.1):
	if c.cfg.EnableNoOpendirSupport && noOpendirSupport {
		initOp.Flags |= fusekernel.InitNoOpendirSupport
	}

	// Tell the Kernel to allow sending parallel lookup and readdir operations.
	if c.cfg.EnableParallelDirOps {
		initOp.Flags |= fusekernel.InitParallelDirOps
	}

	if c.cfg.EnableAtomicTrunc {
		initOp.Flags |= fusekernel.InitAtomicTrunc
	}

	if c.cfg.EnableReaddirplus {
		// Enable Readdirplus support, allowing the kernel to use Readdirplus
		initOp.Flags |= fusekernel.InitDoReaddirplus

		if c.cfg.EnableAutoReaddirplus {
			// Enable adaptive Readdirplus, allowing the kernel to choose between Readdirplus and Readdir
			initOp.Flags |= fusekernel.InitReaddirplusAuto
		}
	}

	return c.Reply(ctx, nil)
}

// Log information for an operation with the given ID. calldepth is the depth
// to use when recovering file:line information with runtime.Caller.
func (c *Connection) debugLog(
	fuseID uint64,
	calldepth int,
	format string,
	v ...interface{}) {
	if c.debugLogger == nil {
		return
	}

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
		fuseID,
		fileLine,
		fmt.Sprintf(format, v...))

	// Print it.
	c.debugLogger.Println(msg)
}

// LOCKS_EXCLUDED(c.mu)
func (c *Connection) recordCancelFunc(
	fuseID uint64,
	f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.cancelFuncs[fuseID]; ok {
		panic(fmt.Sprintf("Already have cancel func for request %v", fuseID))
	}

	c.cancelFuncs[fuseID] = f
}

// Set up state for an op that is about to be returned to the user, given its
// underlying fuse opcode and request ID.
//
// Return a context that should be used for the op.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) beginOp(
	opCode uint32,
	fuseID uint64) context.Context {
	// Start with the parent context.
	ctx := c.cfg.OpContext

	// Set up a cancellation function.
	//
	// Special case: On Darwin, osxfuse aggressively reuses "unique" request IDs.
	// This matters for Forget requests, which have no reply associated and
	// therefore have IDs that are immediately eligible for reuse. For these, we
	// should not record any state keyed on their ID.
	//
	// Cf. https://github.com/osxfuse/osxfuse/issues/208
	if opCode != fusekernel.OpForget {
		var cancel func()
		ctx, cancel = context.WithCancel(ctx)
		c.recordCancelFunc(fuseID, cancel)
	}

	return ctx
}

// Clean up all state associated with an op to which the user has responded,
// given its underlying fuse opcode and request ID. This must be called before
// a response is sent to the kernel, to avoid a race where the request's ID
// might be reused by osxfuse.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) finishOp(
	opCode uint32,
	fuseID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Even though the op is finished, context.WithCancel requires us to arrange
	// for the cancellation function to be invoked. We also must remove it from
	// our map.
	//
	// Special case: we don't do this for Forget requests. See the note in
	// beginOp above.
	if opCode != fusekernel.OpForget {
		cancel, ok := c.cancelFuncs[fuseID]
		if !ok {
			panic(fmt.Sprintf("Unknown request ID in finishOp: %v", fuseID))
		}

		cancel()
		delete(c.cancelFuncs, fuseID)
	}
}

// LOCKS_EXCLUDED(c.mu)
func (c *Connection) handleInterrupt(fuseID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// NOTE(jacobsa): fuse.txt in the Linux kernel documentation
	// (https://tinyurl.com/2r4ajuwd) defines the kernel <-> userspace protocol
	// for interrupts.
	//
	// In particular, my reading of it is that an interrupt request cannot be
	// delivered to userspace before the original request. The part about the
	// race and EAGAIN appears to be aimed at userspace programs that
	// concurrently process requests (https://tinyurl.com/3euehwfb).
	//
	// So in this method if we can't find the ID to be interrupted, it means that
	// the request has already been replied to.
	//
	// Cf. https://github.com/osxfuse/osxfuse/issues/208
	// Cf. http://comments.gmane.org/gmane.comp.file-systems.fuse.devel/14675
	cancel, ok := c.cancelFuncs[fuseID]
	if !ok {
		return
	}

	cancel()
}

// Read the next message from the kernel. The message must later be destroyed
// using destroyInMessage.
func (c *Connection) readMessage() (*buffer.InMessage, error) {
	// Allocate a message.
	m := c.getInMessage()

	// Loop past transient errors.
	for {
		// Attempt a read.
		err := m.Init(c.dev)

		// Special cases:
		//
		//  *  ENODEV means fuse has hung up.
		//
		//  *  EINTR means we should try again. (This seems to happen often on
		//     OS X, cf. http://golang.org/issue/11180)
		//
		if pe, ok := err.(*os.PathError); ok {
			switch pe.Err {
			case syscall.ENODEV:
				err = io.EOF

			case syscall.EINTR:
				err = nil
				continue
			}
		}

		if err != nil {
			c.putInMessage(m)
			return nil, err
		}

		return m, nil
	}
}

// Write a buffer.OutMessage to the kernel, with writev if vectored IO is useful
// and write if not.
func (c *Connection) writeOutMessage(outMsg *buffer.OutMessage) error {
	var err error
	if outMsg.Sglist != nil {
		if fusekernel.IsPlatformFuseT {
			// writev is not atomic on macos, restrict to fuse-t platform
			writeLock.Lock()
			defer writeLock.Unlock()
		}
		_, err = writev(int(c.dev.Fd()), outMsg.Sglist)
	} else {
		err = c.writeMessage(outMsg.OutHeaderBytes())
	}
	return err
}

// Write the supplied message to the kernel.
func (c *Connection) writeMessage(msg []byte) error {
	// Avoid the retry loop in os.File.Write.
	n, err := syscall.Write(int(c.dev.Fd()), msg)
	if err != nil {
		return err
	}

	if n != len(msg) {
		return fmt.Errorf("Wrote %d bytes; expected %d", n, len(msg))
	}

	return nil
}

// ReadOp consumes the next op from the kernel process, returning the op and a
// context that should be used for work related to the op. It returns io.EOF if
// the kernel has closed the connection.
//
// If err != nil, the user is responsible for later calling c.Reply with the
// returned context.
//
// This function delivers ops in exactly the order they are received from
// /dev/fuse. It must not be called multiple times concurrently.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) ReadOp() (_ context.Context, op interface{}, _ error) {
	// Keep going until we find a request we know how to convert.
	for {
		// Read the next message from the kernel.
		inMsg, err := c.readMessage()
		if err != nil {
			return nil, nil, err
		}

		// Convert the message to an op.
		outMsg := c.getOutMessage()
		op, err = convertInMessage(&c.cfg, inMsg, outMsg, c.protocol)
		if err != nil {
			c.putOutMessage(outMsg)
			return nil, nil, fmt.Errorf("convertInMessage: %v", err)
		}

		// Choose an ID for this operation for the purposes of logging, and log it.
		if c.debugLogger != nil {
			c.debugLog(inMsg.Header().Unique, 1, "<- %s", describeRequest(op))
		}

		// Special case: handle interrupt requests inline.
		if interruptOp, ok := op.(*interruptOp); ok {
			c.handleInterrupt(interruptOp.FuseID)
			continue
		}

		// Set up a context that remembers information about this op.
		ctx := c.beginOp(inMsg.Header().Opcode, inMsg.Header().Unique)
		ctx = context.WithValue(ctx, contextKey, opState{inMsg, outMsg, op})

		// Return the op to the user.
		return ctx, op, nil
	}
}

// Skip errors that happen as a matter of course, since they spook users.
func (c *Connection) shouldLogError(
	op interface{},
	err error) bool {
	// We don't log non-errors.
	if err == nil {
		return false
	}

	// We can't log if there's nothing to log to.
	if c.errorLogger == nil {
		return false
	}

	switch op.(type) {
	case *fuseops.LookUpInodeOp:
		// It is totally normal for the kernel to ask to look up an inode by name
		// and find the name doesn't exist. For example, this happens when linking
		// a new file.
		if err == syscall.ENOENT {
			return false
		}
	case *fuseops.GetXattrOp, *fuseops.ListXattrOp:
		if err == syscall.ENOSYS || err == syscall.ENODATA || err == syscall.ERANGE {
			return false
		}
	case *unknownOp:
		// Don't bother the user with methods we intentionally don't support.
		if err == syscall.ENOSYS {
			return false
		}
	}

	return true
}

var writeLock sync.Mutex

// Reply replies to an op previously read using ReadOp, with the supplied error
// (or nil if successful). The context must be the context returned by ReadOp.
//
// LOCKS_EXCLUDED(c.mu)
func (c *Connection) Reply(ctx context.Context, opErr error) error {
	// Extract the state we stuffed in earlier.
	var key interface{} = contextKey
	foo := ctx.Value(key)
	state, ok := foo.(opState)
	if !ok {
		panic(fmt.Sprintf("Reply called with invalid context: %#v", ctx))
	}

	op := state.op
	inMsg := state.inMsg
	outMsg := state.outMsg
	fuseID := inMsg.Header().Unique

	defer func() {
		// Invoke any callbacks set by the FUSE server after the response to the kernel is
		// complete and before the inMessage and outMessage memory buffers have been freed.
		callback := c.callbackForOp(op)
		if callback != nil {
			callback()
		}

		// Make sure we destroy the messages when we're done.
		c.putInMessage(inMsg)
		c.putOutMessage(outMsg)
	}()

	// Clean up state for this op.
	c.finishOp(inMsg.Header().Opcode, inMsg.Header().Unique)

	logError := c.shouldLogError(op, opErr)

	// Debug logging
	if c.debugLogger != nil {
		if opErr == nil {
			c.debugLog(fuseID, 1, "-> %s", describeResponse(op))
		} else {
			if !logError {
				c.debugLog(fuseID, 1, "-> Error: %q", opErr.Error())
			}
		}
	}

	// Error logging
	if logError {
		c.errorLogger.Printf("Op 0x%08x %T] -> Error: %q", fuseID, op, opErr)
	}

	// Send the reply to the kernel, if one is required.
	noResponse := c.kernelResponse(outMsg, inMsg.Header().Unique, op, opErr)

	if !noResponse {
		err := c.writeOutMessage(outMsg)
		if err != nil {
			writeErrMsg := fmt.Sprintf("writeMessage: %v %v", err, outMsg.OutHeaderBytes())
			if c.errorLogger != nil {
				c.errorLogger.Print(writeErrMsg)
			}
			return fmt.Errorf(writeErrMsg)
		}
		outMsg.Sglist = nil
	}

	return nil
}

func (c *Connection) callbackForOp(op interface{}) func() {
	switch o := op.(type) {
	case *fuseops.ReadFileOp:
		return o.Callback
	case *fuseops.WriteFileOp:
		return o.Callback
	}
	return nil
}

// Close the connection. Must not be called until operations that were read
// from the connection have been responded to.
func (c *Connection) close() error {
	// Posix doesn't say that close can be called concurrently with read or
	// write, but luckily we exclude the possibility of a race by requiring the
	// user to respond to all ops first.
	return c.dev.Close()
}
