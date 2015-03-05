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

package memfs

import (
	"fmt"
	"os"
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

	// Set up the root inode. Its ownership information will later be modified in
	// Init.
	rootAttrs := fuse.InodeAttributes{
		Mode: 0700 | os.ModeDir,
	}

	fs.inodes[fuse.RootInodeID] = newInode(rootAttrs)

	// Set up invariant checking.
	fs.mu = syncutil.NewInvariantMutex(fs.checkInvariants)

	return fs
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

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

// Find the given inode and return it with its lock held. Panic if it doesn't
// exist.
//
// SHARED_LOCKS_REQUIRED(fs.mu)
// EXCLUSIVE_LOCK_FUNCTION(inode.mu)
func (fs *memFS) getInodeForModifyingOrDie(id fuse.InodeID) (inode *inode) {
	inode = fs.inodes[id]
	if inode == nil {
		panic(fmt.Sprintf("Unknown inode: %v", id))
	}

	inode.mu.Lock()
	return
}

// Find the given inode and return it with its lock held for reading. Panic if
// it doesn't exist.
//
// SHARED_LOCKS_REQUIRED(fs.mu)
// SHARED_LOCK_FUNCTION(inode.mu)
func (fs *memFS) getInodeForReadingOrDie(id fuse.InodeID) (inode *inode) {
	inode = fs.inodes[id]
	if inode == nil {
		panic(fmt.Sprintf("Unknown inode: %v", id))
	}

	inode.mu.RLock()
	return
}

// Allocate a new inode, assigning it an ID that is not in use. Return it with
// its lock held.
//
// EXCLUSIVE_LOCKS_REQUIRED(fs.mu)
// EXCLUSIVE_LOCK_FUNCTION(inode.mu)
func (fs *memFS) allocateInode(
	attrs fuse.InodeAttributes) (id fuse.InodeID, inode *inode) {
	// Create and lock the inode.
	inode = newInode(attrs)
	inode.mu.Lock()

	// Re-use a free ID if possible. Otherwise mint a new one.
	numFree := len(fs.freeInodes)
	if numFree != 0 {
		id = fs.freeInodes[numFree-1]
		fs.freeInodes = fs.freeInodes[:numFree-1]
		fs.inodes[id] = inode
	} else {
		id = fuse.InodeID(len(fs.inodes))
		fs.inodes = append(fs.inodes, inode)
	}

	return
}

// EXCLUSIVE_LOCKS_REQUIRED(fs.mu)
func (fs *memFS) deallocateInode(id fuse.InodeID) {
	fs.freeInodes = append(fs.freeInodes, id)
	fs.inodes[id] = nil
}

////////////////////////////////////////////////////////////////////////
// FileSystem methods
////////////////////////////////////////////////////////////////////////

func (fs *memFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Update the root inode's ownership information to match the credentials of
	// the mounting process.
	root := fs.getInodeForModifyingOrDie(fuse.RootInodeID)
	defer root.mu.Unlock()

	root.attributes.Uid = req.Header.Uid
	root.attributes.Gid = req.Header.Gid

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
	resp.Entry.Child = childID
	resp.Entry.Attributes = child.attributes

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.Entry.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)
	resp.Entry.EntryExpiration = resp.Entry.EntryExpiration

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

func (fs *memFS) MkDir(
	ctx context.Context,
	req *fuse.MkDirRequest) (resp *fuse.MkDirResponse, err error) {
	resp = &fuse.MkDirResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Grab the parent, which we will update shortly.
	parent := fs.getInodeForModifyingOrDie(req.Parent)
	defer parent.mu.Unlock()

	// Set up attributes from the child, using the credentials of the calling
	// process as owner (matching inode_init_owner, cf. http://goo.gl/5qavg8).
	now := fs.clock.Now()
	childAttrs := fuse.InodeAttributes{
		Mode:   req.Mode,
		Atime:  now,
		Mtime:  now,
		Ctime:  now,
		Crtime: now,
		Uid:    req.Header.Uid,
		Gid:    req.Header.Gid,
	}

	// Allocate a child.
	childID, child := fs.allocateInode(childAttrs)
	defer child.mu.Unlock()

	// Add an entry in the parent.
	parent.AddChild(childID, req.Name, fuseutil.DT_Directory)

	// Fill in the response.
	resp.Entry.Child = childID
	resp.Entry.Attributes = child.attributes

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.Entry.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)
	resp.Entry.EntryExpiration = resp.Entry.EntryExpiration

	return
}

func (fs *memFS) CreateFile(
	ctx context.Context,
	req *fuse.CreateFileRequest) (resp *fuse.CreateFileResponse, err error) {
	resp = &fuse.CreateFileResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Grab the parent, which we will update shortly.
	parent := fs.getInodeForModifyingOrDie(req.Parent)
	defer parent.mu.Unlock()

	// Set up attributes from the child, using the credentials of the calling
	// process as owner (matching inode_init_owner, cf. http://goo.gl/5qavg8).
	now := fs.clock.Now()
	childAttrs := fuse.InodeAttributes{
		Mode:   req.Mode,
		Atime:  now,
		Mtime:  now,
		Ctime:  now,
		Crtime: now,
		Uid:    req.Header.Uid,
		Gid:    req.Header.Gid,
	}

	// Allocate a child.
	childID, child := fs.allocateInode(childAttrs)
	defer child.mu.Unlock()

	// Add an entry in the parent.
	parent.AddChild(childID, req.Name, fuseutil.DT_File)

	// Fill in the response entry.
	resp.Entry.Child = childID
	resp.Entry.Attributes = child.attributes

	// We don't spontaneously mutate, so the kernel can cache as long as it wants
	// (since it also handles invalidation).
	resp.Entry.AttributesExpiration = fs.clock.Now().Add(365 * 24 * time.Hour)
	resp.Entry.EntryExpiration = resp.Entry.EntryExpiration

	// We have nothing interesting to put in the Handle field.

	return
}

func (fs *memFS) RmDir(
	ctx context.Context,
	req *fuse.RmDirRequest) (resp *fuse.RmDirResponse, err error) {
	resp = &fuse.RmDirResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Grab the parent, which we will update shortly.
	parent := fs.getInodeForModifyingOrDie(req.Parent)
	defer parent.mu.Unlock()

	// Find the child within the parent.
	childID, ok := parent.LookUpChild(req.Name)
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Grab the child.
	child := fs.getInodeForModifyingOrDie(childID)
	defer child.mu.Unlock()

	// Make sure the child is empty.
	if child.Len() != 0 {
		err = fuse.ENOTEMPTY
		return
	}

	// Remove the entry within the parent.
	parent.RemoveChild(req.Name)

	// Mark the child as unlinked.
	child.linkCount--

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

	// Serve the request.
	resp.Data, err = inode.ReadDir(int(req.Offset), req.Size)
	if err != nil {
		err = fmt.Errorf("inode.ReadDir: %v", err)
		return
	}

	return
}

func (fs *memFS) WriteFile(
	ctx context.Context,
	req *fuse.WriteFileRequest) (resp *fuse.WriteFileResponse, err error) {
	resp = &fuse.WriteFileResponse{}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Find the inode in question.
	inode := fs.getInodeForModifyingOrDie(req.Inode)
	defer inode.mu.Unlock()

	// Serve the request.
	_, err = inode.WriteAt(req.Data, req.Offset)

	return
}
