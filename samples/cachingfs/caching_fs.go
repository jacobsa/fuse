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
)

// Constants that define the relative offsets of the inodes exported by the
// file system. See notes on the RenumberInodes method.
const (
	FooInodeOffset = iota
	DirInodeOffset
	BarInodeOffset
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
type CachingFS struct {
	fuseutil.NotImplementedFileSystem
}

var _ fuse.FileSystem = &CachingFS{}

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
	getattrTimeout time.Duration) (fs *CachingFS, err error)

// Cause inodes to receive IDs according to the following rules in further
// responses to fuse:
//
//  *  The ID of "foo" is base + FooInodeOffset.
//  *  The ID of "dir" is base + DirInodeOffset.
//  *  The ID of "dir/bar" is base + BarInodeOffset.
//
// If this method has never been called, the file system behaves as if it were
// called with base set to fuse.RootInodeID + 1.
//
// REQUIRES: base > fuse.RootInodeID
func (fs *CachingFS) RenumberInodes(base fuse.InodeID)

// Cause further queries for the attributes of inodes to use the supplied time
// as the inode's mtime.
func (fs *CachingFS) SetMtime(mtime time.Time)
