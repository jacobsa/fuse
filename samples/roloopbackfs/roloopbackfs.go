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

package roloopbackfs

import (
	"golang.org/x/net/context"
	"log"
	"os"
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type readonlyLoopbackFs struct {
	fuseutil.NotImplementedFileSystem
	loopbackPath string
	inodes       *sync.Map
	logger       *log.Logger
}

var _ fuseutil.FileSystem = &readonlyLoopbackFs{}

// Create a file system that mirrors an existing physical path, in a readonly mode

func NewReadonlyLoopbackServer(loopbackPath string, logger *log.Logger) (server fuse.Server, err error) {

	if _, err = os.Stat(loopbackPath); err != nil {
		return nil, err
	}

	inodes := &sync.Map{}
	root := &inodeEntry{
		id:   fuseops.RootInodeID,
		path: loopbackPath,
	}
	inodes.Store(root.Id(), root)
	server = fuseutil.NewFileSystemServer(&readonlyLoopbackFs{
		loopbackPath: loopbackPath,
		inodes:       inodes,
		logger:       logger,
	})
	return
}

func (fs *readonlyLoopbackFs) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	entry, err := getOrCreateInode(fs.inodes, op.Parent, op.Name)
	if err != nil {
		fs.logger.Printf("fs.LookUpInode for '%v' on '%v': %v", entry, op.Name, err)
		return fuse.EIO
	}
	if entry == nil {
		return fuse.ENOENT
	}
	outputEntry := &op.Entry
	outputEntry.Child = entry.Id()
	attributes, err := entry.Attributes()
	if err != nil {
		fs.logger.Printf("fs.LookUpInode.Attributes for '%v' on '%v': %v", entry, op.Name, err)
		return fuse.EIO
	}
	outputEntry.Attributes = *attributes
	return nil
}

func (fs *readonlyLoopbackFs) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	var entry, found = fs.inodes.Load(op.Inode)
	if !found {
		return fuse.ENOENT
	}
	attributes, err := entry.(Inode).Attributes()
	if err != nil {
		fs.logger.Printf("fs.GetInodeAttributes for '%v': %v", entry, err)
		return fuse.EIO
	}
	op.Attributes = *attributes
	return nil
}

func (fs *readonlyLoopbackFs) OpenDir(
	ctx context.Context,
	op *fuseops.OpenDirOp) error {
	// Allow opening any directory.
	return nil
}

func (fs *readonlyLoopbackFs) ReadDir(
	ctx context.Context,
	op *fuseops.ReadDirOp) error {
	var entry, found = fs.inodes.Load(op.Inode)
	if !found {
		return fuse.ENOENT
	}
	children, err := entry.(Inode).ListChildren(fs.inodes)
	if err != nil {
		fs.logger.Printf("fs.ReadDir for '%v': %v", entry, err)
		return fuse.EIO
	}

	if op.Offset > fuseops.DirOffset(len(children)) {
		return nil
	}

	children = children[op.Offset:]

	for _, child := range children {
		bytesWritten := fuseutil.WriteDirent(op.Dst[op.BytesRead:], *child)
		if bytesWritten == 0 {
			break
		}
		op.BytesRead += bytesWritten
	}
	return nil
}

func (fs *readonlyLoopbackFs) OpenFile(
	ctx context.Context,
	op *fuseops.OpenFileOp) error {

	var _, found = fs.inodes.Load(op.Inode)
	if !found {
		return fuse.ENOENT
	}
	return nil

}

func (fs *readonlyLoopbackFs) ReadFile(
	ctx context.Context,
	op *fuseops.ReadFileOp) error {
	var entry, found = fs.inodes.Load(op.Inode)
	if !found {
		return fuse.ENOENT
	}
	contents, err := entry.(Inode).Contents()
	if err != nil {
		fs.logger.Printf("fs.ReadFile for '%v': %v", entry, err)
		return fuse.EIO
	}

	if op.Offset > int64(len(contents)) {
		return fuse.EIO
	}

	contents = contents[op.Offset:]
	op.BytesRead = copy(op.Dst, contents)
	return nil
}

func (fs *readonlyLoopbackFs) ReleaseDirHandle(
	ctx context.Context,
	op *fuseops.ReleaseDirHandleOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) GetXattr(
	ctx context.Context,
	op *fuseops.GetXattrOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) ListXattr(
	ctx context.Context,
	op *fuseops.ListXattrOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) ForgetInode(
	ctx context.Context,
	op *fuseops.ForgetInodeOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) ReleaseFileHandle(
	ctx context.Context,
	op *fuseops.ReleaseFileHandleOp) error {
	return nil
}

func (fs *readonlyLoopbackFs) FlushFile(
	ctx context.Context,
	op *fuseops.FlushFileOp) error {
	return nil
}
