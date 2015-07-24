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

	"github.com/jacobsa/fuse/internal/fuseshim"

	"golang.org/x/net/context"
)

// A type that knows how to serve ops read from a connection.
type Server interface {
	// Read and serve ops from the supplied connection until EOF. Do not return
	// until all operations have been responded to. Must not be called more than
	// once.
	ServeOps(*Connection)
}

// A struct representing the status of a mount operation, with a method that
// waits for unmounting.
type MountedFileSystem struct {
	dir string

	// The result to return from Join. Not valid until the channel is closed.
	joinStatus          error
	joinStatusAvailable chan struct{}
}

// Return the directory on which the file system is mounted (or where we
// attempted to mount it.)
func (mfs *MountedFileSystem) Dir() string {
	return mfs.dir
}

// Block until a mounted file system has been unmounted. Do not return
// successfully until all ops read from the connection have been responded to
// (i.e. the file system server has finished processing all in-flight ops).
//
// The return value will be non-nil if anything unexpected happened while
// serving. May be called multiple times.
func (mfs *MountedFileSystem) Join(ctx context.Context) error {
	select {
	case <-mfs.joinStatusAvailable:
		return mfs.joinStatus
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Attempt to mount a file system on the given directory, using the supplied
// Server to serve connection requests. This function blocks until the file
// system is successfully mounted.
func Mount(
	dir string,
	server Server,
	config *MountConfig) (mfs *MountedFileSystem, err error) {
	// Initialize the struct.
	mfs = &MountedFileSystem{
		dir:                 dir,
		joinStatusAvailable: make(chan struct{}),
	}

	// Open a fuseshim connection.
	bfConn, err := fuseshim.Mount(mfs.dir, config.bazilfuseOptions()...)
	if err != nil {
		err = fmt.Errorf("fuseshim.Mount: %v", err)
		return
	}

	// Choose a parent context for ops.
	opContext := config.OpContext
	if opContext == nil {
		opContext = context.Background()
	}

	// Create our own Connection object wrapping it.
	connection, err := newConnection(
		opContext,
		config.DebugLogger,
		config.ErrorLogger,
		bfConn)

	if err != nil {
		bfConn.Close()
		err = fmt.Errorf("newConnection: %v", err)
		return
	}

	// Serve the connection in the background. When done, set the join status.
	go func() {
		server.ServeOps(connection)
		mfs.joinStatus = connection.close()
		close(mfs.joinStatusAvailable)
	}()

	// Wait for the connection to say it is ready.
	if err = connection.waitForReady(); err != nil {
		err = fmt.Errorf("WaitForReady: %v", err)
		return
	}

	return
}
