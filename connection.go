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
}

// Responsibility for closing the wrapped connection is transferred to the
// result. You must call c.close() eventually.
func newConnection(
	parentCtx context.Context,
	logger *log.Logger,
	wrapped *bazilfuse.Conn) (c *Connection, err error) {
	c = &Connection{
		logger:    logger,
		wrapped:   wrapped,
		parentCtx: parentCtx,
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

// Set up state for an op that is about to be returned to the user.
func (c *Connection) beginOp() {
	c.opsInFlight.Add(1)
}

// Clean up all state associated with an op to which the user has responded.
func (c *Connection) finishOp() {
	c.opsInFlight.Done()
}

// Read the next op from the kernel process. Return io.EOF if the kernel has
// closed the connection.
//
// This function delivers ops in exactly the order they are received from
// /dev/fuse. It must not be called multiple times concurrently.
func (c *Connection) ReadOp() (op fuseops.Op, err error) {
	var bfReq bazilfuse.Request

	// Keep going until we find a request we know how to convert.
	for {
		// Read a bazilfuse request.
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

		// Convert it, if possible.
		logForOp := func(calldepth int, format string, v ...interface{}) {
			c.log(opID, calldepth+1, format, v...)
		}

		finished := func(err error) { c.finishOp() }

		op = fuseops.Convert(c.parentCtx, bfReq, logForOp, finished)
		if op == nil {
			c.log(opID, 1, "-> ENOSYS: %v", bfReq)
			bfReq.RespondError(ENOSYS)
			continue
		}

		c.beginOp()
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
