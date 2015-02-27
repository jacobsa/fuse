// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
)

type memDir struct {
	/////////////////////////
	// Constant data
	/////////////////////////

	inode fuse.InodeID

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The contents of the directory. An entry with inode zero is unused.
	//
	// This array can never be shortened, nor can its elements be moved, because
	// we use its indices for Dirent.Offset, which is exposed to the user who
	// might be calling readdir in a loop while concurrently modifying the
	// directory. Unused entries can, however, be reused.
	entries []fuseutil.Dirent
}
