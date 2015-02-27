// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
	"github.com/jacobsa/gcsfuse/timeutil"
)

// Create a file system that stores data and metadata in memory.
func NewMemFS(
	clock timeutil.Clock) fuse.FileSystem

type memFS struct {
	fuseutil.NotImplementedFileSystem

	/////////////////////////
	// Dependencies
	/////////////////////////

	clock timeutil.Clock

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The next inode to issue.
	//
	// INVARIANT: nextInode > 0
	nextInode fuse.InodeID // GUARDED_BY(mu)

	// A map from inode number to file or directory with that inode.
	//
	// INVARIANT: inodeIndex[fuse.RootInodeID != nil
	// INVARIANT: For all keys k, k > 0
	// INVARIANT: For all keys k, k < nextInode
	// INVARIANT: For all keys k, inodeIndex[k] is *memFile or *memDir
	// INVARIANT: For all keys k, inodeIndex[k].inode == k
	// INVARIANT: For all dirs d, all of d's children are in the map.
	inodeIndex map[fuse.InodeID]interface{} // GUARDED_BY(mu)
}

type memFile struct {
	/////////////////////////
	// Constant data
	/////////////////////////

	inode fuse.InodeID

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu sync.RWMutex

	// The current contents of the file.
	contents []byte // GUARDED_BY(mu)
}

// TODO(jacobsa): Add a test that various WriteAt calls with a real on-disk
// file to verify what the behavior should be here, particularly when starting
// a write well beyond EOF. Leave the test around for documentation purposes.
func (f *memFile) WriteAt(p []byte, off int64) (n int, err error)

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
