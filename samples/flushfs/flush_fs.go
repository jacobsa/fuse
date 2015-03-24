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

package flushfs

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
)

// Create a file system whose sole contents are a file named "foo" and a
// directory named "bar".
//
// The file may be opened for reading and/or writing. Its initial contents are
// empty. Whenever a flush or fsync is received, the supplied function will be
// called with the current contents of the file and its status returned.
//
// The directory cannot be modified.
func NewFileSystem(
	reportFlush func(string) error,
	reportFsync func(string) error) (server fuse.Server, err error) {
	server = &flushFS{
		reportFlush: reportFlush,
		reportFsync: reportFsync,
	}

	return
}

const (
	fooID = fuseops.RootInodeID + 1 + iota
	barID
)

type flushFS struct {
	reportFlush func(string) error
	reportFsync func(string) error

	mu          sync.Mutex
	fooContents []byte // GUARDED_BY(mu)
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) rootAttributes() fuseops.InodeAttributes {
	return fuseops.InodeAttributes{
		Nlink: 1,
		Mode:  0777 | os.ModeDir,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) fooAttributes() fuseops.InodeAttributes {
	return fuseops.InodeAttributes{
		Nlink: 1,
		Mode:  0777,
		Size:  uint64(len(fs.fooContents)),
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) barAttributes() fuseops.InodeAttributes {
	return fuseops.InodeAttributes{
		Nlink: 1,
		Mode:  0777 | os.ModeDir,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) ServeOps(c *fuse.Connection) {
	for {
		op, err := c.ReadOp()
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		switch typed := op.(type) {
		case *fuseops.InitOp:
			fs.init(typed)

		case *fuseops.LookUpInodeOp:
			fs.lookUpInode(typed)

		case *fuseops.GetInodeAttributesOp:
			fs.getInodeAttributes(typed)

		case *fuseops.OpenFileOp:
			fs.openFile(typed)

		case *fuseops.ReadFileOp:
			fs.readFile(typed)

		case *fuseops.WriteFileOp:
			fs.writeFile(typed)

		case *fuseops.SyncFileOp:
			fs.syncFile(typed)

		case *fuseops.FlushFileOp:
			fs.flushFile(typed)

		case *fuseops.OpenDirOp:
			fs.openDir(typed)

		default:
			typed.Respond(fuse.ENOSYS)
		}
	}
}

////////////////////////////////////////////////////////////////////////
// Op methods
////////////////////////////////////////////////////////////////////////

func (fs *flushFS) init(op *fuseops.InitOp) {
	var err error
	defer func() { op.Respond(err) }()

	return
}

func (fs *flushFS) lookUpInode(op *fuseops.LookUpInodeOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Sanity check.
	if op.Parent != fuseops.RootInodeID {
		err = fuse.ENOENT
		return
	}

	// Set up the entry.
	switch op.Name {
	case "foo":
		op.Entry = fuseops.ChildInodeEntry{
			Child:      fooID,
			Attributes: fs.fooAttributes(),
		}

	case "bar":
		op.Entry = fuseops.ChildInodeEntry{
			Child:      barID,
			Attributes: fs.barAttributes(),
		}

	default:
		err = fuse.ENOENT
		return
	}

	return
}

func (fs *flushFS) getInodeAttributes(op *fuseops.GetInodeAttributesOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch op.Inode {
	case fuseops.RootInodeID:
		op.Attributes = fs.rootAttributes()
		return

	case fooID:
		op.Attributes = fs.fooAttributes()
		return

	case barID:
		op.Attributes = fs.barAttributes()
		return

	default:
		err = fuse.ENOENT
		return
	}
}

func (fs *flushFS) openFile(op *fuseops.OpenFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Sanity check.
	if op.Inode != fooID {
		err = fuse.ENOSYS
		return
	}

	return
}

func (fs *flushFS) readFile(op *fuseops.ReadFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Ensure the offset is in range.
	if op.Offset > int64(len(fs.fooContents)) {
		return
	}

	// Read what we can.
	op.Data = make([]byte, op.Size)
	copy(op.Data, fs.fooContents[op.Offset:])

	return
}

func (fs *flushFS) writeFile(op *fuseops.WriteFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Ensure that the contents slice is long enough.
	newLen := int(op.Offset) + len(op.Data)
	if len(fs.fooContents) < newLen {
		padding := make([]byte, newLen-len(fs.fooContents))
		fs.fooContents = append(fs.fooContents, padding...)
	}

	// Copy in the data.
	n := copy(fs.fooContents[op.Offset:], op.Data)

	// Sanity check.
	if n != len(op.Data) {
		panic(fmt.Sprintf("Unexpected short copy: %v", n))
	}

	return
}

func (fs *flushFS) syncFile(op *fuseops.SyncFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	err = fs.reportFsync(string(fs.fooContents))
	return
}

func (fs *flushFS) flushFile(op *fuseops.FlushFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	err = fs.reportFlush(string(fs.fooContents))
	return
}

func (fs *flushFS) openDir(op *fuseops.OpenDirOp) {
	var err error
	defer func() { op.Respond(err) }()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Sanity check.
	if op.Inode != barID {
		err = fuse.ENOSYS
		return
	}

	return
}
