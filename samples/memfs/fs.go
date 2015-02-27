// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
	"github.com/jacobsa/gcsfuse/timeutil"
)

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
	// INVARIANT: inodeIndex[fuse.RootInodeID] != nil
	// INVARIANT: For all keys k, k > 0
	// INVARIANT: For all keys k, k < nextInode
	// INVARIANT: For all keys k, inodeIndex[k] is *memFile or *memDir
	// INVARIANT: For all keys k, inodeIndex[k].inode == k
	// INVARIANT: For all dirs d, all of d's children are in the map.
	inodeIndex map[fuse.InodeID]interface{} // GUARDED_BY(mu)
}

// Create a file system that stores data and metadata in memory.
func NewMemFS(
	clock timeutil.Clock) fuse.FileSystem
