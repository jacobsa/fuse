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
	"io"
	"os"
	"time"

	"github.com/googlecloudplatform/gcsfuse/timeutil"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
)

// Common attributes for files and directories.
type inode struct {
	/////////////////////////
	// Dependencies
	/////////////////////////

	clock timeutil.Clock

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The current attributes of this inode.
	//
	// INVARIANT: attrs.Mode &^ (os.ModePerm|os.ModeDir|os.ModeSymlink) == 0
	// INVARIANT: !(isDir() && isSymlink())
	// INVARIANT: attrs.Size == len(contents)
	attrs fuseops.InodeAttributes // GUARDED_BY(mu)

	// For directories, entries describing the children of the directory. Unused
	// entries are of type DT_Unknown.
	//
	// This array can never be shortened, nor can its elements be moved, because
	// we use its indices for Dirent.Offset, which is exposed to the user who
	// might be calling readdir in a loop while concurrently modifying the
	// directory. Unused entries can, however, be reused.
	//
	// INVARIANT: If !isDir(), len(entries) == 0
	// INVARIANT: For each i, entries[i].Offset == i+1
	// INVARIANT: Contains no duplicate names in used entries.
	entries []fuseutil.Dirent // GUARDED_BY(mu)

	// For files, the current contents of the file.
	//
	// INVARIANT: If !isFile(), len(contents) == 0
	contents []byte // GUARDED_BY(mu)

	// For symlinks, the target of the symlink.
	//
	// INVARIANT: If !isSymlink(), len(target) == 0
	//
	// GUARDED_BY(mu)
	target string
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

// Create a new inode with the supplied attributes, which need not contain
// time-related information (the inode object will take care of that).
func newInode(
	clock timeutil.Clock,
	attrs fuseops.InodeAttributes) (in *inode) {
	// Update time info.
	now := clock.Now()
	attrs.Mtime = now
	attrs.Crtime = now

	// Create the object.
	in = &inode{
		clock:      clock,
		dir:        (attrs.Mode&os.ModeDir != 0),
		attributes: attrs,
	}

	in.mu = syncutil.NewInvariantMutex(in.checkInvariants)
	return
}

func (in *inode) checkInvariants() {
	// INVARIANT: attrs.Mode &^ (os.ModePerm|os.ModeDir|os.ModeSymlink) == 0
	if !(in.attrs.Mode&^(os.ModePerm|os.ModeDir|os.ModeSymlink) == 0) {
		panic(fmt.Sprintf("Unexpected mode: %v", in.attrs.Mode))
	}

	// INVARIANT: !(isDir() && isSymlink())
	if in.isDir() && in.isSymlink() {
		panic(fmt.Sprintf("Unexpected mode: %v", in.attrs.Mode))
	}

	// INVARIANT: attrs.Size == len(contents)
	if in.attrs.Size != len(in.contents) {
		panic(fmt.Sprintf(
			"Size mismatch: %d vs. %d",
			in.attrs.Size,
			len(in.contents)))
	}

	// INVARIANT: If !isDir(), len(entries) == 0
	if !in.isDir() && len(entries) != 0 {
		panic(fmt.Sprintf("Unexpected entries length: %d", len(entries)))
	}

	// INVARIANT: For each i, entries[i].Offset == i+1
	for i, e := range in.entries {
		if !(e.Offset == i+1) {
			panic(fmt.Sprintf("Unexpected offset for index %d: %d", i, e.Offset))
		}
	}

	// INVARIANT: Contains no duplicate names in used entries.
	childNames := make(map[string]struct{})
	for i, e := range inode.entries {
		if e.Type != fuseutil.DT_Unknown {
			if _, ok := childNames[e.Name]; ok {
				panic(fmt.Sprintf("Duplicate name: %s", e.Name))
			}

			childNames[e.Name] = struct{}{}
		}
	}

	// INVARIANT: If !isFile(), len(contents) == 0
	if !in.isFile() && len(in.contents) != 0 {
		panic(fmt.Sprintf("Unexpected length: %d", len(in.contents)))
	}

	// INVARIANT: If !isSymlink(), len(target) == 0
	if !in.isSymlink() && len(in.target) != 0 {
		panic(fmt.Sprintf("Unexpected target length: %d", len(in.target)))
	}

	return
}

// LOCKS_REQUIRED(in.mu)
func (in *inode) isDir() bool {
	return in.attrs.Mode&os.ModeDir != 0
}

// LOCKS_REQUIRED(in.mu)
func (in *inode) isSymlink() bool {
	return in.attrs.Mode&os.ModeSymlink != 0
}

// LOCKS_REQUIRED(in.mu)
func (in *inode) isFile() bool {
	return !(in.isDir() || in.isSymlink())
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
func (inode *inode) LookUpChild(name string) (id fuseops.InodeID, ok bool) {
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
	id fuseops.InodeID,
	name string,
	dt fuseutil.DirentType) {
	var index int

	// Update the modification time.
	inode.attributes.Mtime = inode.clock.Now()

	// No matter where we place the entry, make sure it has the correct Offset
	// field.
	defer func() {
		inode.entries[index].Offset = fuseops.DirOffset(index + 1)
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
	// Update the modification time.
	inode.attributes.Mtime = inode.clock.Now()

	// Find the entry.
	i, ok := inode.findChild(name)
	if !ok {
		panic(fmt.Sprintf("Unknown child: %s", name))
	}

	// Mark it as unused.
	inode.entries[i] = fuseutil.Dirent{
		Type:   fuseutil.DT_Unknown,
		Offset: fuseops.DirOffset(i + 1),
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

// Read from the file's contents. See documentation for ioutil.ReaderAt.
//
// REQUIRES: !inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) ReadAt(p []byte, off int64) (n int, err error) {
	if inode.dir {
		panic("ReadAt called on directory.")
	}

	// Ensure the offset is in range.
	if off > int64(len(inode.contents)) {
		err = io.EOF
		return
	}

	// Read what we can.
	n = copy(p, inode.contents[off:])
	if n < len(p) {
		err = io.EOF
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

	// Update the modification time.
	inode.attributes.Mtime = inode.clock.Now()

	// Ensure that the contents slice is long enough.
	newLen := int(off) + len(p)
	if len(inode.contents) < newLen {
		padding := make([]byte, newLen-len(inode.contents))
		inode.contents = append(inode.contents, padding...)
		inode.attributes.Size = uint64(newLen)
	}

	// Copy in the data.
	n = copy(inode.contents[off:], p)

	// Sanity check.
	if n != len(p) {
		panic(fmt.Sprintf("Unexpected short copy: %v", n))
	}

	return
}

// Update attributes from non-nil parameters.
//
// EXCLUSIVE_LOCKS_REQUIRED(inode.mu)
func (inode *inode) SetAttributes(
	size *uint64,
	mode *os.FileMode,
	mtime *time.Time) {
	// Update the modification time.
	inode.attributes.Mtime = inode.clock.Now()

	// Truncate?
	if size != nil {
		intSize := int(*size)

		// Update contents.
		if intSize <= len(inode.contents) {
			inode.contents = inode.contents[:intSize]
		} else {
			padding := make([]byte, intSize-len(inode.contents))
			inode.contents = append(inode.contents, padding...)
		}

		// Update attributes.
		inode.attributes.Size = *size
	}

	// Change mode?
	if mode != nil {
		inode.attributes.Mode = *mode
	}

	// Change mtime?
	if mtime != nil {
		inode.attributes.Mtime = *mtime
	}
}
