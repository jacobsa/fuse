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

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
)

// Common attributes for files and directories.
//
// TODO(jacobsa): Add tests for interacting with a file/directory after it has
// been unlinked, including creating a new file. Make sure we don't screw up
// and reuse an inode ID while it is still in use.
type inode struct {
	/////////////////////////
	// Constant data
	/////////////////////////

	// Is this a directory? If not, it is a file.
	dir bool

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The number of times this inode is linked into a parent directory. This may
	// be zero if the inode has been unlinked but not yet forgotten, because some
	// process still has an  open file handle.
	//
	// INVARIANT: linkCount >= 0
	linkCount int // GUARDED_BY(mu)

	// The current attributes of this inode.
	//
	// INVARIANT: No non-permission mode bits are set besides os.ModeDir
	// INVARIANT: If dir, then os.ModeDir is set
	// INVARIANT: If !dir, then os.ModeDir is not set
	attributes fuse.InodeAttributes // GUARDED_BY(mu)

	// For directories, entries describing the children of the directory. Unused
	// entries are of type DT_Unknown.
	//
	// This array can never be shortened, nor can its elements be moved, because
	// we use its indices for Dirent.Offset, which is exposed to the user who
	// might be calling readdir in a loop while concurrently modifying the
	// directory. Unused entries can, however, be reused.
	//
	// TODO(jacobsa): Add good tests exercising concurrent modifications while
	// doing readdir, seekdir, etc. calls.
	//
	// INVARIANT: If dir is false, this is nil.
	// INVARIANT: For each i, entries[i].Offset == i+1
	// INVARIANT: Contains no duplicate names in used entries.
	entries []fuseutil.Dirent // GUARDED_BY(mu)

	// For files, the current contents of the file.
	//
	// INVARIANT: If dir is true, this is nil.
	contents []byte // GUARDED_BY(mu)
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

// Initially the link count is one.
func newInode(attrs fuse.InodeAttributes) (in *inode) {
	in = &inode{
		linkCount:  1,
		dir:        (attrs.Mode&os.ModeDir != 0),
		attributes: attrs,
	}

	in.mu = syncutil.NewInvariantMutex(in.checkInvariants)
	return
}

func (inode *inode) checkInvariants() {
	// Check the link count.
	if inode.linkCount < 0 {
		panic(fmt.Sprintf("Negative link count: %v", inode.linkCount))
	}

	// No non-permission mode bits should be set besides os.ModeDir.
	if inode.attributes.Mode & ^(os.ModePerm|os.ModeDir) != 0 {
		panic(fmt.Sprintf("Unexpected mode: %v", inode.attributes.Mode))
	}

	// Check os.ModeDir.
	if inode.dir != (inode.attributes.Mode&os.ModeDir == os.ModeDir) {
		panic(
			fmt.Sprintf(
				"Unexpected mode: %v, dir: %v",
				inode.attributes.Mode,
				inode.dir))
	}

	// Check directory-specific stuff.
	if inode.dir {
		if inode.contents != nil {
			panic("Non-nil contents in a directory.")
		}

		childNames := make(map[string]struct{})
		for i, e := range inode.entries {
			if e.Offset != fuse.DirOffset(i+1) {
				panic(fmt.Sprintf("Unexpected offset: %v", e.Offset))
			}

			if e.Type != fuseutil.DT_Unknown {
				if _, ok := childNames[e.Name]; ok {
					panic(fmt.Sprintf("Duplicate name: %s", e.Name))
				}

				childNames[e.Name] = struct{}{}
			}
		}
	}

	// Check file-specific stuff.
	if !inode.dir {
		if inode.entries != nil {
			panic("Non-nil entries in a file.")
		}
	}
}

// Return the index of the child within inode.entries, if it exists.
//
// REQUIRES: inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) findChild(name string) (i int, ok bool) {
	if !inode.dir {
		panic("findChild called on non-directory.")
	}

	var e fuseutil.Dirent
	for i, e = range inode.entries {
		if e.Name == name {
			ok = true
			return
		}
	}

	return
}

////////////////////////////////////////////////////////////////////////
// Public methods
////////////////////////////////////////////////////////////////////////

// Return the number of children of the directory.
//
// REQUIRES: inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) Len() (n int) {
	for _, e := range inode.entries {
		if e.Type != fuseutil.DT_Unknown {
			n++
		}
	}

	return
}

// Find an entry for the given child name and return its inode ID.
//
// REQUIRES: inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) LookUpChild(name string) (id fuse.InodeID, ok bool) {
	index, ok := inode.findChild(name)
	if ok {
		id = inode.entries[index].Inode
	}

	return
}

// Add an entry for a child.
//
// REQUIRES: inode.dir
// REQUIRES: dt != fuseutil.DT_Unknown
// EXCLUSIVE_LOCKS_REQUIRED(inode.mu)
func (inode *inode) AddChild(
	id fuse.InodeID,
	name string,
	dt fuseutil.DirentType) {
	var index int

	// No matter where we place the entry, make sure it has the correct Offset
	// field.
	defer func() {
		inode.entries[index].Offset = fuse.DirOffset(index + 1)
	}()

	// Set up the entry.
	e := fuseutil.Dirent{
		Inode: id,
		Name:  name,
		Type:  dt,
	}

	// Look for a gap in which we can insert it.
	for index = range inode.entries {
		if inode.entries[index].Type == fuseutil.DT_Unknown {
			inode.entries[index] = e
			return
		}
	}

	// Append it to the end.
	index = len(inode.entries)
	inode.entries = append(inode.entries, e)
}

// Remove an entry for a child.
//
// REQUIRES: inode.dir
// REQUIRES: An entry for the given name exists.
// EXCLUSIVE_LOCKS_REQUIRED(inode.mu)
func (inode *inode) RemoveChild(name string) {
	// Find the entry.
	i, ok := inode.findChild(name)
	if !ok {
		panic(fmt.Sprintf("Unknown child: %s", name))
	}

	// Mark it as unused.
	inode.entries[i] = fuseutil.Dirent{
		Type:   fuseutil.DT_Unknown,
		Offset: fuse.DirOffset(i + 1),
	}
}

// Serve a ReadDir request.
//
// REQUIRES: inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) ReadDir(offset int, size int) (data []byte, err error) {
	if !inode.dir {
		panic("ReadDir called on non-directory.")
	}

	for i := offset; i < len(inode.entries); i++ {
		e := inode.entries[i]

		// Skip unused entries.
		if e.Type == fuseutil.DT_Unknown {
			continue
		}

		data = fuseutil.AppendDirent(data, inode.entries[i])

		// Trim and stop early if we've exceeded the requested size.
		if len(data) > size {
			data = data[:size]
			break
		}
	}

	return
}

// Write to the file's contents. See documentation for ioutil.WriterAt.
//
// REQUIRES: !inode.dir
// EXCLUSIVE_LOCKS_REQUIRED(inode.mu)
func (inode *inode) WriteAt(p []byte, off int64) (n int, err error) {
	if inode.dir {
		panic("WriteAt called on directory.")
	}

	// Ensure that the contents slice is long enough.
	newLen := int(off) + len(p)
	if len(inode.contents) < newLen {
		padding := make([]byte, newLen-len(inode.contents))
		inode.contents = append(inode.contents, padding...)
	}

	// Copy in the data.
	n = copy(inode.contents[off:], p)

	// Sanity check.
	if n != len(p) {
		panic(fmt.Sprintf("Unexpected short copy: %v", n))
	}

	return
}
