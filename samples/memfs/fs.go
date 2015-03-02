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

	// The collection of all inodes that have ever been created, indexed by inode
	// ID. Some inodes are not in use if they have been unlinked, and no inode
	// with ID less than fuse.RootInodeID is ever used.
	//
	// INVARIANT: len(inodes) > fuse.RootInodeID
	// INVARIANT: For all i < fuse.RootInodeID, inodes[i].impl == nil
	// INVARIANT: inodes[fuse.RootInodeID].impl is of type *memDir
	inodes []inode // GUARDED_BY(mu)

	// A list of inode IDs within inodes available for reuse, not including the
	// reserved IDs less than fuse.RootInodeID.
	//
	// INVARIANT: This is all and only indices i of inodes such that i >
	// fuse.RootInodeID and inodes[i].impl == nil
	freeInodes []fuse.InodeID // GUARDED_BY(mu)
}

// Create a file system that stores data and metadata in memory.
func NewMemFS(
	clock timeutil.Clock) fuse.FileSystem {
	panic("TODO(jacobsa): Implement NewMemFS.")
}