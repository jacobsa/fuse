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

package killprivfs

import (
	"context"
	"os"
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

// KillPrivFS is a simple filesystem that tracks when KillSuidgid flags are received.
type KillPrivFS struct {
	fuseutil.NotImplementedFileSystem

	mu                     sync.Mutex
	createWithKillSuidgid  bool
	openWithKillSuidgid    bool
	writeWithKillSuidgid   bool
	setattrWithKillSuidgid bool
	fileData               []byte // Simple in-memory file storage
}

// NewKillPrivFS creates a new KillPrivFS.
func NewKillPrivFS() *KillPrivFS {
	return &KillPrivFS{}
}

func (fs *KillPrivFS) GetFlags() (create, open, write, setattr bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.createWithKillSuidgid, fs.openWithKillSuidgid, fs.writeWithKillSuidgid, fs.setattrWithKillSuidgid
}

func (fs *KillPrivFS) ResetFlags() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.createWithKillSuidgid = false
	fs.openWithKillSuidgid = false
	fs.writeWithKillSuidgid = false
	fs.setattrWithKillSuidgid = false
}

func (fs *KillPrivFS) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}

func (fs *KillPrivFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	if op.Inode == fuseops.RootInodeID {
		op.Attributes = fuseops.InodeAttributes{
			Mode:  os.ModeDir | 0755,
			Nlink: 1,
		}
		return nil
	}

	if op.Inode == 2 {
		fs.mu.Lock()
		size := uint64(len(fs.fileData))
		fs.mu.Unlock()

		op.Attributes = fuseops.InodeAttributes{
			Mode:  0666, // Allow all permissions for testing
			Nlink: 1,
			Size:  size,
		}
		return nil
	}

	return fuse.ENOENT
}

func (fs *KillPrivFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	if op.Parent == fuseops.RootInodeID {
		op.Entry.Child = 2
		op.Entry.Attributes = fuseops.InodeAttributes{
			Mode:  0666,
			Nlink: 1,
			Size:  0,
		}
		return nil
	}

	return fuse.ENOENT
}

func (fs *KillPrivFS) CreateFile(
	ctx context.Context,
	op *fuseops.CreateFileOp) error {
	fs.mu.Lock()
	if op.KillSuidgid {
		fs.createWithKillSuidgid = true
	}
	fs.mu.Unlock()

	// Return a new inode
	op.Entry.Child = 2
	op.Entry.Attributes = fuseops.InodeAttributes{
		Mode:  op.Mode,
		Nlink: 1,
		Size:  0,
	}
	op.Handle = 1
	return nil
}

func (fs *KillPrivFS) OpenFile(
	ctx context.Context,
	op *fuseops.OpenFileOp) error {
	fs.mu.Lock()
	if op.KillSuidgid {
		fs.openWithKillSuidgid = true
	}
	fs.mu.Unlock()

	op.Handle = 1
	return nil
}

func (fs *KillPrivFS) WriteFile(
	ctx context.Context,
	op *fuseops.WriteFileOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if op.KillSuidgid {
		fs.writeWithKillSuidgid = true
	}

	if op.Offset+int64(len(op.Data)) > int64(len(fs.fileData)) {
		// Extend file
		newSize := op.Offset + int64(len(op.Data))
		newData := make([]byte, newSize)
		copy(newData, fs.fileData)
		fs.fileData = newData
	}
	copy(fs.fileData[op.Offset:], op.Data)

	return nil
}

func (fs *KillPrivFS) SetInodeAttributes(
	ctx context.Context,
	op *fuseops.SetInodeAttributesOp) error {
	fs.mu.Lock()
	if op.KillSuidgid {
		fs.setattrWithKillSuidgid = true
	}
	fs.mu.Unlock()

	mode := os.FileMode(0666)
	if op.Mode != nil {
		mode = *op.Mode
	}
	size := uint64(0)
	if op.Size != nil {
		size = *op.Size
	}

	op.Attributes = fuseops.InodeAttributes{
		Mode:  mode,
		Nlink: 1,
		Size:  size,
	}
	return nil
}

func (fs *KillPrivFS) ReadFile(
	ctx context.Context,
	op *fuseops.ReadFileOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Handle read beyond file size
	if op.Offset >= int64(len(fs.fileData)) {
		op.BytesRead = 0
		return nil
	}

	n := copy(op.Dst, fs.fileData[op.Offset:])
	op.BytesRead = n
	return nil
}

func (fs *KillPrivFS) ReleaseFileHandle(
	ctx context.Context,
	op *fuseops.ReleaseFileHandleOp) error {
	return nil
}
