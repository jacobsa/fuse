// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"fmt"
	"reflect"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
	"github.com/jacobsa/gcsfuse/timeutil"
	"golang.org/x/net/context"
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
	clock timeutil.Clock) (fs fuse.FileSystem) {
	fs = &memFS{
		clock: clock,
	}

	fs.(*memFS).mu = syncutil.NewInvariantMutex(fs.(*memFS).checkInvariants)
	return
}

func (fs *memFS) checkInvariants() {
	// Check reserved inodes.
	for i := 0; i < fuse.RootInodeID; i++ {
		var inode *inode = &fs.inodes[i]
		if inode.impl != nil {
			panic(fmt.Sprintf("Non-nil impl for ID: %v", i))
		}
	}

	// Check the root inode.
	fs.inodes[fuse.RootInodeID].impl.(*dir)

	// Check inodes, building our own set of free IDs.
	freeIDsEncountered := make(map[fuse.InodeID]struct{})
	for i := range fs.inodes {
		var inode *inode = &fs.inodes[i]
		if inode.impl == nil {
			freeIDsEncountered[i] = struct{}{}
			continue
		}

		// Check for known types.
		switch inode.impl.(type) {
		case *memFile:
		case *memDir:
		default:
			panic(fmt.Sprintf("Unknown inode type: %v", reflect.TypeOf(inode.impl)))
		}
	}

	panic("TODO")
}

func (fs *memFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}

func (fs *memFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (resp *fuse.OpenDirResponse, err error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// We don't mutate spontaneosuly, so if the VFS layer has asked for an
	// inode that doesn't exist, something screwed up earlier (a lookup, a
	// cache invalidation, etc.).
	if req.Inode >= fuse.InodeID(len(fs.inodes)) {
		panic(fmt.Sprintf("Inode out of range: %v vs. %v", req.Inode, len(fs.inodes)))
	}

	var inode *inode = &fs.inodes[req.Inode]
	if inode.impl == nil {
		panic(fmt.Sprintf("Dead inode requested: %v", req.Inode))
	}

	// All is good.
	return
}
