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
	cfs := &cachingFS{
		baseID: (fuse.RootInodeID + 1 + numInodes - 1) / numInodes,
		mtime:  time.Now(),
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
	mu syncutil.InvariantMutex

	// The current ID of the lowest numbered non-root inode.
	//
	// INVARIANT: FooID() > fuse.RootInodeID
	// INVARIANT: DirID() > fuse.RootInodeID
	// INVARIANT: BarID() > fuse.RootInodeID
	//
	// INVARIANT: FooID() % numInodes == fooOffset
	// INVARIANT: DirID() % numInodes == dirOffset
	// INVARIANT: BarID() % numInodes == barOffset
	//
	// GUARDED_BY(mu)
	baseID fuse.InodeID

	// GUARDED_BY(mu)
	mtime time.Time
}

func (fs *cachingFS) checkInvariants()

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) FooID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.baseID + fooOffset
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) DirID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.baseID + dirOffset
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) BarID() fuse.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.baseID + barOffset
}

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) RenumberInodes()

// LOCKS_EXCLUDED(fs.mu)
func (fs *cachingFS) SetMtime(mtime time.Time) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.mtime = mtime
}

func (fs *cachingFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}
