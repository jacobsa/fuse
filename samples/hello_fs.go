// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package samples

import (
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcsfuse/timeutil"
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
			// TODO(jacobsa): Why do we get premission denied errors when this is
			// 0500?
			Mode: 0555 | os.ModeDir,
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
			Mode: 0400,
			Size: uint64(len("Hello, world!")),
		},
	},

	// dir
	dirInode: inodeInfo{
		attributes: fuse.InodeAttributes{
			Mode: 0500 | os.ModeDir,
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
			Mode: 0400,
			Size: uint64(len("Hello, world!")),
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
	resp.Child = childInode
	resp.Attributes = gInodeInfo[childInode].attributes

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

	return
}

func (fs *HelloFS) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (resp *fuse.OpenDirResponse, err error) {
	// We always allow opening the root directory.
	if req.Inode == rootInode {
		resp = &fuse.OpenDirResponse{}
		return
	}

	// TODO(jacobsa): Handle others.
	err = fuse.ENOSYS
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
