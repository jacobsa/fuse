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
	"runtime"

	"github.com/jacobsa/bazilfuse"
	"golang.org/x/net/context"
)

// A type that knows how to serve ops read from a connection.
type Server interface {
	// Read and serve ops from the supplied connection until EOF.
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

// Block until a mounted file system has been unmounted. The return value will
// be non-nil if anything unexpected happened while serving. May be called
// multiple times.
func (mfs *MountedFileSystem) Join(ctx context.Context) error {
	select {
	case <-mfs.joinStatusAvailable:
		return mfs.joinStatus
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Optional configuration accepted by Mount.
type MountConfig struct {
	// OS X only.
	//
	// Normally on OS X we mount with the novncache option
	// (cf. http://goo.gl/1pTjuk), which disables entry caching in the kernel.
	// This is because osxfuse does not honor the entry expiration values we
	// return to it, instead caching potentially forever (cf.
	// http://goo.gl/8yR0Ie), and it is probably better to fail to cache than to
	// cache for too long, since the latter is more likely to hide consistency
	// bugs that are difficult to detect and diagnose.
	//
	// This field disables the use of novncache, restoring entry caching. Beware:
	// the value of ChildInodeEntry.EntryExpiration is ignored by the kernel, and
	// entries will be cached for an arbitrarily long time.
	EnableVnodeCaching bool
}

// Convert to mount options to be passed to package bazilfuse.
func (c *MountConfig) bazilfuseOptions() (opts []bazilfuse.MountOption) {
	isDarwin := runtime.GOOS == "darwin"

	// Enable permissions checking in the kernel. See the comments on
	// InodeAttributes.Mode.
	opts = append(opts, bazilfuse.SetOption("default_permissions", ""))

	// OS X: set novncache when appropriate.
	if isDarwin && !c.EnableVnodeCaching {
		opts = append(opts, bazilfuse.SetOption("novncache", ""))
	}

	// OS X: disable the use of "Apple Double" (._foo and .DS_Store) files, which
	// just add noise to debug output and can have significant cost on
	// network-based file systems.
	//
	// Cf. https://github.com/osxfuse/osxfuse/wiki/Mount-options
	if isDarwin {
		opts = append(opts, bazilfuse.SetOption("noappledouble", ""))
	}

	return
}

// Attempt to mount a file system on the given directory, using the supplied
// Server to serve connection requests. This function blocks until the file
// system is successfully mounted. On some systems, this requires the supplied
// Server to make forward progress (in particular, to respond to
// fuseops.InitOp).
func Mount(
	dir string,
	server Server,
	config *MountConfig) (mfs *MountedFileSystem, err error) {
	logger := getLogger()

	// Initialize the struct.
	mfs = &MountedFileSystem{
		dir:                 dir,
		joinStatusAvailable: make(chan struct{}),
	}

	// Open a bazilfuse connection.
	logger.Println("Opening a bazilfuse connection.")
	bfConn, err := bazilfuse.Mount(mfs.dir, config.bazilfuseOptions()...)
	if err != nil {
		err = fmt.Errorf("bazilfuse.Mount: %v", err)
		return
	}

	// Create our own Connection object wrapping it.
	connection, err := newConnection(logger, bfConn)
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
