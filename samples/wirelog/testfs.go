// Copyright 2025 Google Inc. All Rights Reserved.
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

package wirelog

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

// NewTestFS returns a simple file system with a root directory and one file "foo".
func NewTestFS() fuse.Server {
	return fuseutil.NewFileSystemServer(&testFS{})
}

type testFS struct {
	fuseutil.NotImplementedFileSystem
}

const (
	rootInode fuseops.InodeID = fuseops.RootInodeID + iota
	fileInode
)

var fileName string = "foo"
var fileContents string = "bar"
var fileMode os.FileMode = 0444
var fileHandle fuseops.HandleID = 10

func (fs *testFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	if wlog := fuse.GetWirelog(ctx); wlog != nil {
		wlog.Extra["lookup"] = "yes"
	}
	if op.Parent == rootInode && op.Name == fileName {
		op.Entry.Child = fileInode
		op.Entry.Attributes = fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  fileMode,
			Size:  uint64(len(fileContents)),
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *testFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	switch op.Inode {
	case rootInode:
		op.Attributes = fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		}
	case fileInode:
		op.Attributes = fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  fileMode,
			Size:  uint64(len(fileContents)),
		}
	default:
		return fuse.ENOENT
	}
	return nil
}

func (fs *testFS) OpenDir(
	ctx context.Context,
	op *fuseops.OpenDirOp) error {
	if op.Inode == rootInode {
		return nil
	}
	return fuse.ENOENT
}

func (fs *testFS) ReadDir(
	ctx context.Context,
	op *fuseops.ReadDirOp) error {
	if op.Inode != rootInode {
		return fuse.ENOENT
	}

	entries := []fuseutil.Dirent{
		{
			Offset: 1,
			Inode:  fileInode,
			Name:   fileName,
			Type:   fuseutil.DT_File,
		},
	}

	if op.Offset > fuseops.DirOffset(len(entries)) {
		return nil
	}

	for _, e := range entries[op.Offset:] {
		n := fuseutil.WriteDirent(op.Dst[op.BytesRead:], e)
		if n == 0 {
			break
		}
		op.BytesRead += n
	}
	return nil
}

func (fs *testFS) OpenFile(
	ctx context.Context,
	op *fuseops.OpenFileOp) error {
	if op.Inode == fileInode {
		op.Handle = fileHandle
		return nil
	}
	return fuse.ENOENT
}

func (fs *testFS) FlushFile(
	ctx context.Context,
	op *fuseops.FlushFileOp) error {
	return nil
}

func (fs *testFS) ReadFile(
	ctx context.Context,
	op *fuseops.ReadFileOp) error {
	if op.Inode != fileInode {
		return fuse.ENOENT
	}
	reader := strings.NewReader(fileContents)
	var err error
	op.BytesRead, err = reader.ReadAt(op.Dst, op.Offset)
	if err == io.EOF {
		return nil
	}
	return err
}
