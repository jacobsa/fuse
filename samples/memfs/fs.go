// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"fmt"
	"time"

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

	// When acquiring this lock, the caller must hold no inode locks.
	mu syncutil.InvariantMutex

	// The collection of live inodes, indexed by ID. IDs of free inodes that may
	// be re-used have nil entries. No ID less than fuse.RootInodeID is ever used.
	//
	// INVARIANT: len(inodes) > fuse.RootInodeID
	// INVARIANT: For all i < fuse.RootInodeID, inodes[i] == nil
	// INVARIANT: inodes[fuse.RootInodeID] != nil
	// INVARIANT: inodes[fuse.RootInodeID].dir is true
	inodes []*inode // GUARDED_BY(mu)

	// A list of inode IDs within inodes available for reuse, not including the
	// reserved IDs less than fuse.RootInodeID.
	//
	// INVARIANT: This is all and only indices i of 'inodes' such that i >
	// fuse.RootInodeID and inodes[i] == nil
	freeInodes []fuse.InodeID // GUARDED_BY(mu)
}

// Create a file system that stores data and metadata in memory.
func NewMemFS(
	clock timeutil.Clock) fuse.FileSystem {
	// Set up the basic struct.
	fs := &memFS{
		clock:  clock,
		inodes: make([]*inode, fuse.RootInodeID+1),
	}

	// Set up the root inode.
	fs.inodes[fuse.RootInodeID] = newInode(true) // dir

	// Set up invariant checking.
	fs.mu = syncutil.NewInvariantMutex(fs.checkInvariants)

	return fs
}

func (fs *memFS) checkInvariants() {
	// Check reserved inodes.
	for i := 0; i < fuse.RootInodeID; i++ {
		if fs.inodes[i] != nil {
			panic(fmt.Sprintf("Non-nil inode for ID: %v", i))
		}
	}

	// Check the root inode.
	if !fs.inodes[fuse.RootInodeID].dir {
		panic("Expected root to be a directory.")
	}

	// Build our own list of free IDs.
	freeIDsEncountered := make(map[fuse.InodeID]struct{})
	for i := fuse.RootInodeID + 1; i < len(fs.inodes); i++ {
		inode := fs.inodes[i]
		if inode == nil {
			freeIDsEncountered[fuse.InodeID(i)] = struct{}{}
			continue
		}
	}

	// Check fs.freeInodes.
	if len(fs.freeInodes) != len(freeIDsEncountered) {
		panic(
			fmt.Sprintf(
				"Length mismatch: %v vs. %v",
				len(fs.freeInodes),
				len(freeIDsEncountered)))
	}

	for _, id := range fs.freeInodes {
		if _, ok := freeIDsEncountered[id]; !ok {
			panic(fmt.Sprintf("Unexected free inode ID: %v", id))
		}
	}
}

func (fs *memFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}

// Find the supplied inode and return it with its lock held for reading. Panic
// if it doesn't exist.
//
// SHARED_LOCKS_REQUIRED(fs.mu)
// SHARED_LOCK_FUNCTION(inode.mu)
func (fs *memFS) getInodeForReadingOrDie(id fuse.InodeID) (inode *inode) {
	inode = fs.inodes[id]
	if inode == nil {
		panic(fmt.Sprintf("Unknown inode: %v", id))
	}

	return
}

func (fs *memFS) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (resp *fuse.LookUpInodeResponse, err error) {
	resp = &fuse.LookUpInodeResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Grab the parent directory.
	inode := fs.getInodeForReadingOrDie(req.Parent)
	defer inode.mu.RUnlock()

	// Does the directory have an entry with the given name?
	childID, ok := inode.LookUpChild(req.Name)
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Grab the child.
	child := fs.getInodeForReadingOrDie(childID)
	defer child.mu.RUnlock()

	// Fill in the response.
	resp.Attributes = child.attributes

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)
	resp.EntryExpiration = resp.EntryExpiration

	return
}

func (fs *memFS) GetInodeAttributes(
	ctx context.Context,
	req *fuse.GetInodeAttributesRequest) (
	resp *fuse.GetInodeAttributesResponse, err error) {
	resp = &fuse.GetInodeAttributesResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Grab the inode.
	inode := fs.getInodeForReadingOrDie(req.Inode)
	defer inode.mu.RUnlock()

	// Fill in the response.
	resp.Attributes = inode.attributes

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)

	return
}

func (fs *memFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (resp *fuse.OpenDirResponse, err error) {
	resp = &fuse.OpenDirResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// We don't mutate spontaneosuly, so if the VFS layer has asked for an
	// inode that doesn't exist, something screwed up earlier (a lookup, a
	// cache invalidation, etc.).
	inode := fs.getInodeForReadingOrDie(req.Inode)
	defer inode.mu.RUnlock()

	if !inode.dir {
		panic("Found non-dir.")
	}

	return
}

func (fs *memFS) ReadDir(
	ctx context.Context,
	req *fuse.ReadDirRequest) (resp *fuse.ReadDirResponse, err error) {
	resp = &fuse.ReadDirResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Grab the directory.
	inode := fs.getInodeForReadingOrDie(req.Inode)
	defer inode.mu.RUnlock()

	if !inode.dir {
		panic("Found non-dir.")
	}

	// Serve the request.
	resp.Data, err = inode.ReadDir(int(req.Offset), req.Size)
	if err != nil {
		err = fmt.Errorf("inode.ReadDir: %v", err)
		return
	}

	return
}
