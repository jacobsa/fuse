// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package samples

import (
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/gcsfuse/timeutil"
	"golang.org/x/net/context"
)

// A file system with a fixed structure that looks like this:
//
//     hello
//     dir/
//         world
//
// Each file contains the string "Hello, world!".
type HelloFS struct {
	fuse.NotImplementedFileSystem
	Clock timeutil.Clock
}

var _ fuse.FileSystem = &HelloFS{}

func (fs *HelloFS) Open(
	ctx context.Context,
	req *fuse.OpenRequest) (resp *fuse.OpenResponse, err error) {
	// We always allow opening the root directory.
	if req.Inode == fuse.RootInodeID {
		return
	}

	// TODO(jacobsa): Handle others.
	err = fuse.ENOSYS
	return
}
