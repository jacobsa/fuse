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

package cachingfs

import (
	"fmt"
	"os"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
	"golang.org/x/net/context"
)

const (
	// Sizes of the files according to the file system.
	FooSize = 123
	BarSize = 456
)

// A file system with a fixed structure that looks like this:
//
//     foo
//     dir/
//         bar
//
// The file system is configured with durations that specify how long to allow
// inode entries and attributes to be cached, used when responding to fuse
// requests. It also exposes methods for renumbering inodes and updating mtimes
// that are useful in testing that these durations are honored.
type CachingFS interface {
	fuse.FileSystem

	// Return the current inode ID of the file/directory with the given name.
	FooID() fuse.InodeID
	DirID() fuse.InodeID
	BarID() fuse.InodeID

	// Cause the inode IDs to change to values that have never before been used.
	RenumberInodes()

	// Cause further queries for the attributes of inodes to use the supplied
	// time as the inode's mtime.
	SetMtime(mtime time.Time)
}

// Create a file system that issues cacheable responses according to the
// following rules:
//
//  *  LookUpInodeResponse.Entry.EntryExpiration is set according to
//     lookupEntryTimeout.
//
//  *  GetInodeAttributesResponse.AttributesExpiration is set according to
//     getattrTimeout.
//
//  *  Nothing else is marked cacheable. (In particular, the attributes
//     returned by LookUpInode are not cacheable.)
//
func NewCachingFS(
	lookupEntryTimeout time.Duration,
	getattrTimeout time.Duration) (fs CachingFS, err error) {
	roundUp := func(n fuse.InodeID) fuse.InodeID {
		return numInodes * ((n + numInodes - 1) / numInodes)
	}

	cfs := &cachingFS{
		lookupEntryTimeout: lookupEntryTimeout,
		getattrTimeout:     getattrTimeout,
		baseID:             roundUp(fuse.RootInodeID + 1),
		mtime:              time.Now(),
	}

	cfs.mu = syncutil.NewInvariantMutex(cfs.checkInvariants)

	fs = cfs
	return
}

const (
	// Inode IDs are issued such that "foo" always receives an ID that is
	// congruent to fooOffset modulo numInodes, etc.
	fooOffset = iota
	dirOffset
	barOffset
	numInodes
)

type cachingFS struct {
	fuseutil.NotImplementedFileSystem

	/////////////////////////
	// Constant data
	/////////////////////////

	lookupEntryTimeout time.Duration
	getattrTimeout     time.Duration

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The current ID of the lowest numbered non-root inode.
	//
	// INVARIANT: baseID > fuse.RootInodeID
	// INVARIANT: baseID % numInodes == 0
	//
	// GUARDED_BY(mu)
	baseID fuse.InodeID

	// GUARDED_BY(mu)
	mtime time.Time
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

func (fs *cachingFS) checkInvariants() {
	// INVARIANT: baseID > fuse.RootInodeID
	// INVARIANT: baseID % numInodes == 0
	if fs.baseID <= fuse.RootInodeID || fs.baseID%numInodes != 0 {
		panic(fmt.Sprintf("Bad baseID: %v", fs.baseID))
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) fooID() fuse.InodeID {
	return fs.baseID + fooOffset
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) dirID() fuse.InodeID {
	return fs.baseID + dirOffset
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) barID() fuse.InodeID {
	return fs.baseID + barOffset
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) rootAttrs() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Mode:  os.ModeDir | 0777,
		Mtime: fs.mtime,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) fooAttrs() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Nlink: 1,
		Size:  FooSize,
		Mode:  0777,
		Mtime: fs.mtime,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) dirAttrs() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Nlink: 1,
		Mode:  os.ModeDir | 0777,
		Mtime: fs.mtime,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *cachingFS) barAttrs() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Nlink: 1,
		Size:  BarSize,
		Mode:  0777,
		Mtime: fs.mtime,
	}
}

////////////////////////////////////////////////////////////////////////
// Public interface
////////////////////////////////////////////////////////////////////////

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) FooID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.fooID()
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) DirID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.dirID()
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) BarID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.barID()
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) RenumberInodes() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.baseID += numInodes
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) SetMtime(mtime time.Time) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.mtime = mtime
}

////////////////////////////////////////////////////////////////////////
// FileSystem methods
////////////////////////////////////////////////////////////////////////

func (fs *cachingFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (resp *fuse.LookUpInodeResponse, err error) {
	resp = &fuse.LookUpInodeResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Find the ID and attributes.
	var id fuse.InodeID
	var attrs fuse.InodeAttributes

	switch req.Name {
	case "foo":
		// Parent must be the root.
		if req.Parent != fuse.RootInodeID {
			err = fuse.ENOENT
			return
		}

		id = fs.fooID()
		attrs = fs.fooAttrs()

	case "dir":
		// Parent must be the root.
		if req.Parent != fuse.RootInodeID {
			err = fuse.ENOENT
			return
		}

		id = fs.dirID()
		attrs = fs.dirAttrs()

	case "bar":
		// Parent must be dir.
		if req.Parent == fuse.RootInodeID || req.Parent%numInodes != dirOffset {
			err = fuse.ENOENT
			return
		}

		id = fs.barID()
		attrs = fs.barAttrs()

	default:
		err = fuse.ENOENT
		return
	}

	// Fill in the response.
	resp.Entry.Child = id
	resp.Entry.Attributes = attrs
	resp.Entry.EntryExpiration = time.Now().Add(fs.lookupEntryTimeout)

	return
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) GetInodeAttributes(
	ctx context.Context,
	req *fuse.GetInodeAttributesRequest) (
	resp *fuse.GetInodeAttributesResponse, err error) {
	resp = &fuse.GetInodeAttributesResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Figure out which inode the request is for.
	var attrs fuse.InodeAttributes

	switch {
	case req.Inode == fuse.RootInodeID:
		attrs = fs.rootAttrs()

	case req.Inode%numInodes == fooOffset:
		attrs = fs.fooAttrs()

	case req.Inode%numInodes == dirOffset:
		attrs = fs.dirAttrs()

	case req.Inode%numInodes == barOffset:
		attrs = fs.barAttrs()
	}

	// Fill in the response.
	resp.Attributes = attrs
	resp.AttributesExpiration = time.Now().Add(fs.getattrTimeout)

	return
}

func (fs *cachingFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (
	resp *fuse.OpenDirResponse, err error) {
	resp = &fuse.OpenDirResponse{}
	return
}

func (fs *cachingFS) OpenFile(
	ctx context.Context,
	req *fuse.OpenFileRequest) (
	resp *fuse.OpenFileResponse, err error) {
	resp = &fuse.OpenFileResponse{}
	return
}
