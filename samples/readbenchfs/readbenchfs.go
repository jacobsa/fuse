// Copyright 2021 Vitaliy Filippov
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

package readbenchfs

import (
	"golang.org/x/net/context"
	"io"
	"math/rand"
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type readBenchFS struct {
	fuseutil.NotImplementedFileSystem
	buf []byte
}

// 1 TB
const fileSize = 1024 * 1024 * 1024 * 1024

var _ fuseutil.FileSystem = &readBenchFS{}

// Create a file system that mirrors an existing physical path, in a readonly mode

func NewReadBenchServer() (server fuse.Server, err error) {
	// 1 GB of random data to exceed CPU cache
	buf := make([]byte, 1024*1024*1024)
	rand.Read(buf)
	server = fuseutil.NewFileSystemServer(&readBenchFS{
		buf: buf,
	})
	return
}

func (fs *readBenchFS) StatFS(ctx context.Context, op *fuseops.StatFSOp) error {
	return nil
}

func (fs *readBenchFS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	if op.Name == "test" {
		op.Entry = fuseops.ChildInodeEntry{
			Child: 2,
			Attributes: fuseops.InodeAttributes{
				Size:  fileSize,
				Nlink: 1,
				Mode:  0444,
			},
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *readBenchFS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	if op.Inode == 1 {
		op.Attributes = fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0755 | os.ModeDir,
		}
		return nil
	} else if op.Inode == 2 {
		op.Attributes = fuseops.InodeAttributes{
			Size:  fileSize,
			Nlink: 1,
			Mode:  0444,
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *readBenchFS) OpenDir(ctx context.Context, op *fuseops.OpenDirOp) error {
	// Allow opening any directory.
	return nil
}

func (fs *readBenchFS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	if op.Inode != 1 {
		return fuse.ENOENT
	}
	if op.Offset > 0 {
		return nil
	}
	entries := []fuseutil.Dirent{
		fuseutil.Dirent{
			Offset: 1,
			Inode:  2,
			Name:   "test",
			Type:   fuseutil.DT_File,
		},
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

func (fs *readBenchFS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	// Allow opening any file.
	return nil
}

func (fs *readBenchFS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	if op.Offset > fileSize {
		return io.EOF
	}
	end := op.Offset + int64(len(op.Dst))
	if end > fileSize {
		end = fileSize
	}
	buflen := int64(len(fs.buf))
	for pos := op.Offset; pos < end; {
		s := pos % buflen
		e := buflen
		if e-s > end-pos {
			e = s + end - pos
		}
		copy(op.Dst[pos-op.Offset:], fs.buf[s:])
		pos = op.Offset + e
	}
	//op.Data = [][]byte{ contents[op.Offset : end] }
	op.BytesRead = int(end - op.Offset)
	return nil
}

func (fs *readBenchFS) ReleaseDirHandle(ctx context.Context, op *fuseops.ReleaseDirHandleOp) error {
	return nil
}

func (fs *readBenchFS) GetXattr(ctx context.Context, op *fuseops.GetXattrOp) error {
	return nil
}

func (fs *readBenchFS) ListXattr(ctx context.Context, op *fuseops.ListXattrOp) error {
	return nil
}

func (fs *readBenchFS) ForgetInode(ctx context.Context, op *fuseops.ForgetInodeOp) error {
	return nil
}

func (fs *readBenchFS) ReleaseFileHandle(ctx context.Context, op *fuseops.ReleaseFileHandleOp) error {
	return nil
}

func (fs *readBenchFS) FlushFile(ctx context.Context, op *fuseops.FlushFileOp) error {
	return nil
}
