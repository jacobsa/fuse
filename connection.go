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
	"log"
	"sync"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/fuse/fuseops"
)

// A connection to the fuse kernel process.
type Connection struct {
	logger      *log.Logger
	wrapped     *bazilfuse.Conn
	opsInFlight sync.WaitGroup
	nextOpID    uint64
}

// Responsibility for closing the wrapped connection is transferred to the
// result. You must call c.close() eventually.
func newConnection(
	logger *log.Logger,
	wrapped *bazilfuse.Conn) (c *Connection, err error) {
	c = &Connection{
		logger:  logger,
		wrapped: wrapped,
	}

	return
}

// Log information for an operation with the given unique ID.
func (c *Connection) log(
	opID uint64,
	format string,
	v ...interface{}) {
	// TODO(jacobsa): Add op ID and fixed-width file:line to output.
	c.logger.Printf(format, v...)
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
		c.log(opID, "Received: %v", bfReq)

		// Special case: responding to this is required to make mounting work on OS
		// X. We don't currently expose the capability for the file system to
		// intercept this.
		if statfsReq, ok := bfReq.(*bazilfuse.StatfsRequest); ok {
			c.log(opID, "Responding OK to Statfs.")
			statfsReq.Respond(&bazilfuse.StatfsResponse{})
			continue
		}

		// Convert it, if possible.
		if op = fuseops.Convert(bfReq, c.logger, &c.opsInFlight); op == nil {
			c.log(opID, "Returning ENOSYS for unknown bazilfuse request: %v", bfReq)
			bfReq.RespondError(ENOSYS)
			continue
		}

		c.opsInFlight.Add(1)
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
