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
	// Set up the basic struct.
	fs := &memFS{
		clock:  clock,
		inodes: make([]inode, fuse.RootInodeID+1),
	}

	// Set up the root inode.
	fs.inodes[fuse.RootInodeID].impl = newDir()

	// Set up invariant checking.
	fs.mu = syncutil.NewInvariantMutex(fs.checkInvariants)

	return fs
}

func (fs *memFS) checkInvariants() {
	// Check general inode invariants.
	for i := range fs.inodes {
		fs.inodes[i].checkInvariants()
	}

	// Check reserved inodes.
	for i := 0; i < fuse.RootInodeID; i++ {
		var inode *inode = &fs.inodes[i]
		if inode.impl != nil {
			panic(fmt.Sprintf("Non-nil impl for ID: %v", i))
		}
	}

	// Check the root inode.
	_ = fs.inodes[fuse.RootInodeID].impl.(*memDir)

	// Check inodes, building our own set of free IDs.
	freeIDsEncountered := make(map[fuse.InodeID]struct{})
	for i := fuse.RootInodeID + 1; i < len(fs.inodes); i++ {
		var inode *inode = &fs.inodes[i]
		if inode.impl == nil {
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

// Panic if out of range.
//
// LOCKS_EXCLUDED(fs.mu)
func (fs *memFS) getInodeOrDie(inodeID fuse.InodeID) (inode *inode) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	inode = &fs.inodes[inodeID]
	return
}

// Panic if not a live dir.
//
// LOCKS_EXCLUDED(fs.mu)
func (fs *memFS) getDirOrDie(inodeID fuse.InodeID) (d *memDir) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if inodeID >= fuse.InodeID(len(fs.inodes)) {
		panic(fmt.Sprintf("Inode out of range: %v vs. %v", inodeID, len(fs.inodes)))
	}

	var inode *inode = &fs.inodes[inodeID]
	d = inode.impl.(*memDir)

	return
}

func (fs *memFS) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (resp *fuse.LookUpInodeResponse, err error) {
	resp = &fuse.LookUpInodeResponse{}

	// Grab the parent directory.
	d := fs.getDirOrDie(req.Parent)

	// Does the directory have an entry with the given name?
	childID, ok := d.LookUpInode(req.Name)
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Look up the child.
	child := fs.getInodeOrDie(childID)

	// Fill in the response.
	resp.Attributes = child.Attributes()

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)
	resp.EntryExpiration = resp.EntryExpiration

	return
}

func (fs *memFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (resp *fuse.OpenDirResponse, err error) {
	resp = &fuse.OpenDirResponse{}

	// We don't mutate spontaneosuly, so if the VFS layer has asked for an
	// inode that doesn't exist, something screwed up earlier (a lookup, a
	// cache invalidation, etc.).
	_ = fs.getDirOrDie(req.Inode)

	return
}

func (fs *memFS) ReadDir(
	ctx context.Context,
	req *fuse.ReadDirRequest) (resp *fuse.ReadDirResponse, err error) {
	resp = &fuse.ReadDirResponse{}

	// Grab the directory.
	d := fs.getDirOrDie(req.Inode)

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return the entries requested.
	for i := int(req.Offset); i < len(d.entries); i++ {
		resp.Data = fuseutil.AppendDirent(resp.Data, d.entries[i])

		// Trim and stop early if we've exceeded the requested size.
		if len(resp.Data) > req.Size {
			resp.Data = resp.Data[:req.Size]
			break
		}
	}

	return
}
