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

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/googlecloudplatform/gcsfuse/timeutil"
	"golang.org/x/net/context"
)

// A file system with a fixed structure that looks like this:
//
//     hello
//     dir/
//         world
//
// Each file contains the string "Hello, world!".
type HelloFS struct {
	fuseutil.NotImplementedFileSystem
	Clock timeutil.Clock
}

var _ fuse.FileSystem = &HelloFS{}

const (
	rootInode fuse.InodeID = fuse.RootInodeID + iota
	helloInode
	dirInode
	worldInode
)

type inodeInfo struct {
	attributes fuse.InodeAttributes

	// File or directory?
	dir bool

	// For directories, children.
	children []fuseutil.Dirent
}

// We have a fixed directory structure.
var gInodeInfo = map[fuse.InodeID]inodeInfo{
	// root
	rootInode: inodeInfo{
		attributes: fuse.InodeAttributes{
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
		attributes: fuse.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},

	// dir
	dirInode: inodeInfo{
		attributes: fuse.InodeAttributes{
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
		attributes: fuse.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},
}

func findChildInode(
	name string,
	children []fuseutil.Dirent) (inode fuse.InodeID, err error) {
	for _, e := range children {
		if e.Name == name {
			inode = e.Inode
			return
		}
	}

	err = fuse.ENOENT
	return
}

func (fs *HelloFS) patchAttributes(
	attr *fuse.InodeAttributes) {
	now := fs.Clock.Now()
	attr.Atime = now
	attr.Mtime = now
	attr.Crtime = now
}

func (fs *HelloFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (
	resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}

func (fs *HelloFS) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (
	resp *fuse.LookUpInodeResponse, err error) {
	resp = &fuse.LookUpInodeResponse{}

	// Find the info for the parent.
	parentInfo, ok := gInodeInfo[req.Parent]
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Find the child within the parent.
	childInode, err := findChildInode(req.Name, parentInfo.children)
	if err != nil {
		return
	}

	// Copy over information.
	resp.Entry.Child = childInode
	resp.Entry.Attributes = gInodeInfo[childInode].attributes

	// Patch attributes.
	fs.patchAttributes(&resp.Entry.Attributes)

	return
}

func (fs *HelloFS) GetInodeAttributes(
	ctx context.Context,
	req *fuse.GetInodeAttributesRequest) (
	resp *fuse.GetInodeAttributesResponse, err error) {
	resp = &fuse.GetInodeAttributesResponse{}

	// Find the info for this inode.
	info, ok := gInodeInfo[req.Inode]
	if !ok {
		err = fuse.ENOENT
		return
	}

	// Copy over its attributes.
	resp.Attributes = info.attributes

	// Patch attributes.
	fs.patchAttributes(&resp.Attributes)

	return
}

func (fs *HelloFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (resp *fuse.OpenDirResponse, err error) {
	// Allow opening any directory.
	resp = &fuse.OpenDirResponse{}
	return
}

func (fs *HelloFS) ReadDir(
	ctx context.Context,
	req *fuse.ReadDirRequest) (resp *fuse.ReadDirResponse, err error) {
	resp = &fuse.ReadDirResponse{}

	// Find the info for this inode.
	info, ok := gInodeInfo[req.Inode]
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
	if req.Offset > fuse.DirOffset(len(entries)) {
		err = fuse.EIO
		return
	}

	entries = entries[req.Offset:]

	// Resume at the specified offset into the array.
	for _, e := range entries {
		resp.Data = fuseutil.AppendDirent(resp.Data, e)
		if len(resp.Data) > req.Size {
			resp.Data = resp.Data[:req.Size]
			break
		}
	}

	return
}

func (fs *HelloFS) OpenFile(
	ctx context.Context,
	req *fuse.OpenFileRequest) (resp *fuse.OpenFileResponse, err error) {
	// Allow opening any file.
	resp = &fuse.OpenFileResponse{}
	return
}

func (fs *HelloFS) ReadFile(
	ctx context.Context,
	req *fuse.ReadFileRequest) (resp *fuse.ReadFileResponse, err error) {
	resp = &fuse.ReadFileResponse{}

	// Let io.ReaderAt deal with the semantics.
	reader := strings.NewReader("Hello, world!")

	resp.Data = make([]byte, req.Size)
	n, err := reader.ReadAt(resp.Data, req.Offset)
	resp.Data = resp.Data[:n]

	// Special case: FUSE doesn't expect us to return io.EOF.
	if err == io.EOF {
		err = nil
	}

	return
}
