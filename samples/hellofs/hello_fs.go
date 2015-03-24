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

package hellofs

import (
	"io"
	"os"
	"strings"

	"github.com/googlecloudplatform/gcsfuse/timeutil"
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

// Create a file system with a fixed structure that looks like this:
//
//     hello
//     dir/
//         world
//
// Each file contains the string "Hello, world!".
func NewHelloFS(clock timeutil.Clock) (server fuse.Server, err error) {
	fs := &helloFS{
		Clock: clock,
	}

	server = fuse.Server(fs.serve)
	return
}

type helloFS struct {
	Clock timeutil.Clock
}

func (fs *helloFS) serve(c *fuse.Connection) {
	for {
		op, err := c.ReadOp()
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		switch typed := op.(type) {
		default:
			typed.Respond(fuse.ENOSYS)
		}
	}
}

const (
	rootInode fuseops.InodeID = fuseops.RootInodeID + iota
	helloInode
	dirInode
	worldInode
)

type inodeInfo struct {
	attributes fuseops.InodeAttributes

	// File or directory?
	dir bool

	// For directories, children.
	children []fuseutil.Dirent
}

// We have a fixed directory structure.
var gInodeInfo = map[fuseops.InodeID]inodeInfo{
	// root
	rootInode: inodeInfo{
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		},
		dir: true,
		children: []fuseutil.Dirent{
			fuseutil.Dirent{
				Offset: 1,
				Inode:  helloInode,
				Name:   "hello",
				Type:   fuseutil.DT_File,
			},
			fuseutil.Dirent{
				Offset: 2,
				Inode:  dirInode,
				Name:   "dir",
				Type:   fuseutil.DT_Directory,
			},
		},
	},

	// hello
	helloInode: inodeInfo{
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},

	// dir
	dirInode: inodeInfo{
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		},
		dir: true,
		children: []fuseutil.Dirent{
			fuseutil.Dirent{
				Offset: 1,
				Inode:  worldInode,
				Name:   "world",
				Type:   fuseutil.DT_File,
			},
		},
	},

	// world
	worldInode: inodeInfo{
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},
}

func findChildInode(
	name string,
	children []fuseutil.Dirent) (inode fuseops.InodeID, err error) {
	for _, e := range children {
		if e.Name == name {
			inode = e.Inode
			return
		}
	}

	err = fuse.ENOENT
	return
}

func (fs *helloFS) patchAttributes(
	attr *fuseops.InodeAttributes) {
	now := fs.Clock.Now()
	attr.Atime = now
	attr.Mtime = now
	attr.Crtime = now
}

func (fs *helloFS) init(op *fuseops.InitOp) {
	op.Respond(nil)
}

func (fs *helloFS) lookUpInode(op *fuseops.LookUpInodeOp) {
	var err error
	defer func() { op.Respond(err) }()

	// Find the info for the parent.
	parentInfo, ok := gInodeInfo[op.Parent]
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Find the child within the parent.
	childInode, err := findChildInode(op.Name, parentInfo.children)
	if err != nil {
		return
	}

	// Copy over information.
	op.Entry.Child = childInode
	op.Entry.Attributes = gInodeInfo[childInode].attributes

	// Patch attributes.
	fs.patchAttributes(&op.Entry.Attributes)

	return
}

func (fs *helloFS) getInodeAttributes(op *fuseops.GetInodeAttributesOp) {
	var err error
	defer func() { op.Respond(err) }()

	// Find the info for this inode.
	info, ok := gInodeInfo[op.Inode]
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Copy over its attributes.
	op.Attributes = info.attributes

	// Patch attributes.
	fs.patchAttributes(&op.Attributes)

	return
}

func (fs *helloFS) openDir(op *fuseops.OpenDirOp) {
	// Allow opening any directory.
	op.Respond(nil)
}

func (fs *helloFS) readDir(op fuseops.ReadDirOp) {
	var err error
	defer func() { op.Respond(err) }()

	// Find the info for this inode.
	info, ok := gInodeInfo[op.Inode]
	if !ok {
		err = fuse.ENOENT
		return
	}

	if !info.dir {
		err = fuse.EIO
		return
	}

	entries := info.children

	// Grab the range of interest.
	if op.Offset > fuseops.DirOffset(len(entries)) {
		err = fuse.EIO
		return
	}

	entries = entries[op.Offset:]

	// Resume at the specified offset into the array.
	for _, e := range entries {
		op.Data = fuseutil.AppendDirent(op.Data, e)
		if len(op.Data) > op.Size {
			op.Data = op.Data[:op.Size]
			break
		}
	}

	return
}

func (fs *helloFS) openFile(op *fuseops.OpenFileOp) {
	// Allow opening any file.
	op.Respond(nil)
}

func (fs *helloFS) readFile(op *fuseops.ReadFileOp) {
	var err error
	defer func() { op.Respond(err) }()

	// Let io.ReaderAt deal with the semantics.
	reader := strings.NewReader("Hello, world!")

	op.Data = make([]byte, op.Size)
	n, err := reader.ReadAt(op.Data, op.Offset)
	op.Data = op.Data[:n]

	// Special case: FUSE doesn't expect us to return io.EOF.
	if err == io.EOF {
		err = nil
	}

	return
}
