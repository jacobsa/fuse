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

package forgetfs

import (
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
)

// Create a file system whose sole contents are a file named "foo" and a
// directory named "bar".
//
// The file "foo" may be opened for reading and/or writing, but reads and
// writes aren't supported. Additionally, any non-existent file or directory
// name may be created within any directory, but the resulting inode will
// appear to have been unlinked immediately.
//
// The file system maintains reference counts for the inodes involved. It will
// panic if a reference count becomes negative or if an inode ID is re-used
// after we expect it to be dead. Its Check method may be used to check that
// there are no inodes with non-zero reference counts remaining, after
// unmounting.
func NewFileSystem() (fs *ForgetFS) {
	impl := &fsImpl{}

	fs = &ForgetFS{
		impl:   impl,
		server: fuseutil.NewFileSystemServer(impl),
	}

	return
}

////////////////////////////////////////////////////////////////////////
// ForgetFS
////////////////////////////////////////////////////////////////////////

type ForgetFS struct {
	impl   *fsImpl
	server fuse.Server
}

func (fs *ForgetFS) ServeOps(c *fuse.Connection) {
	fs.server.ServeOps(c)
}

// Panic if there are any inodes that have a non-zero reference count. For use
// after unmounting.
func (fs *ForgetFS) Check() {
	panic("TODO")
}

////////////////////////////////////////////////////////////////////////
// Actual implementation
////////////////////////////////////////////////////////////////////////

type fsImpl struct {
	fuseutil.NotImplementedFileSystem
}
