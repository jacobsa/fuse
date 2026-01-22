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
	"time"

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
	fileData               []byte              // Simple in-memory file storage
	inodes                 map[uint64]inodeInfo // inode storage
	nextInode              uint64
}

type inodeInfo struct {
	mode     os.FileMode
	parent   uint64
	name     string
	children map[string]uint64
}

func NewKillPrivFS() *KillPrivFS {
	fs := &KillPrivFS{
		inodes:    make(map[uint64]inodeInfo),
		nextInode: 2, // Start after root (inode 1)
	}
	// Initialize root directory
	fs.inodes[1] = inodeInfo{
		mode:     os.ModeDir | 0755,
		children: make(map[string]uint64),
	}
	return fs
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

// AddTestFile bypasses normal FUSE operations to create test files with specific mode bits.
func (fs *KillPrivFS) AddTestFile(name string, mode os.FileMode) fuseops.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	inodeID := fs.nextInode
	fs.nextInode++

	fs.inodes[inodeID] = inodeInfo{
		mode:   mode,
		parent: 1, // root
		name:   name,
	}

	rootInfo := fs.inodes[1]
	rootInfo.children[name] = inodeID
	fs.inodes[1] = rootInfo

	return fuseops.InodeID(inodeID)
}

// AddTestDir bypasses normal FUSE operations to create test directories with specific mode bits.
func (fs *KillPrivFS) AddTestDir(name string, mode os.FileMode) fuseops.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	inodeID := fs.nextInode
	fs.nextInode++

	fs.inodes[inodeID] = inodeInfo{
		mode:     mode | os.ModeDir,
		parent:   1, // root
		name:     name,
		children: make(map[string]uint64),
	}

	rootInfo := fs.inodes[1]
	rootInfo.children[name] = inodeID
	fs.inodes[1] = rootInfo

	return fuseops.InodeID(inodeID)
}

func (fs *KillPrivFS) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}

func (fs *KillPrivFS) OpenDir(
	ctx context.Context,
	op *fuseops.OpenDirOp) error {
	op.Handle = 1
	return nil
}

func (fs *KillPrivFS) ReadDir(
	ctx context.Context,
	op *fuseops.ReadDirOp) error {
	// Return empty directory listing for simplicity
	return nil
}

func (fs *KillPrivFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	info, ok := fs.inodes[uint64(op.Inode)]
	if !ok {
		return fuse.ENOENT
	}

	size := uint64(0)
	if info.mode.IsRegular() {
		size = uint64(len(fs.fileData))
	}

	now := time.Now()
	op.Attributes = fuseops.InodeAttributes{
		Mode:  info.mode,
		Nlink: 1,
		Size:  size,
		Uid:   0,
		Gid:   0,
		Atime: now,
		Mtime: now,
		Ctime: now,
	}
	return nil
}

func (fs *KillPrivFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	parentInfo, ok := fs.inodes[uint64(op.Parent)]
	if !ok {
		return fuse.ENOENT
	}

	childInode, ok := parentInfo.children[op.Name]
	if !ok {
		return fuse.ENOENT
	}

	childInfo := fs.inodes[childInode]
	size := uint64(0)
	if childInfo.mode.IsRegular() {
		size = uint64(len(fs.fileData))
	}

	now := time.Now()
	op.Entry.Child = fuseops.InodeID(childInode)
	op.Entry.Attributes = fuseops.InodeAttributes{
		Mode:  childInfo.mode,
		Nlink: 1,
		Size:  size,
		Uid:   0,
		Gid:   0,
		Atime: now,
		Mtime: now,
		Ctime: now,
	}
	return nil
}

func (fs *KillPrivFS) MkDir(
	ctx context.Context,
	op *fuseops.MkDirOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	parentInfo, ok := fs.inodes[uint64(op.Parent)]
	if !ok {
		return fuse.ENOENT
	}

	newInode := fs.nextInode
	fs.nextInode++

	fs.inodes[newInode] = inodeInfo{
		mode:     op.Mode | os.ModeDir,
		parent:   uint64(op.Parent),
		name:     op.Name,
		children: make(map[string]uint64),
	}

	parentInfo.children[op.Name] = newInode
	fs.inodes[uint64(op.Parent)] = parentInfo

	now := time.Now()
	op.Entry.Child = fuseops.InodeID(newInode)
	op.Entry.Attributes = fuseops.InodeAttributes{
		Mode:  op.Mode | os.ModeDir,
		Nlink: 1,
		Uid:   0,
		Gid:   0,
		Atime: now,
		Mtime: now,
		Ctime: now,
	}
	return nil
}

func (fs *KillPrivFS) CreateFile(
	ctx context.Context,
	op *fuseops.CreateFileOp) error {
	fs.mu.Lock()
	if op.KillSuidgid {
		fs.createWithKillSuidgid = true
	}

	parentInfo, ok := fs.inodes[uint64(op.Parent)]
	if !ok {
		fs.mu.Unlock()
		return fuse.ENOENT
	}

	newInode := fs.nextInode
	fs.nextInode++

	// Ensure mode has at least user read/write permissions
	mode := op.Mode
	if mode&0600 == 0 {
		mode |= 0600
	}

	fs.inodes[newInode] = inodeInfo{
		mode:   mode,
		parent: uint64(op.Parent),
		name:   op.Name,
	}

	parentInfo.children[op.Name] = newInode
	fs.inodes[uint64(op.Parent)] = parentInfo
	fs.mu.Unlock()

	now := time.Now()
	op.Entry.Child = fuseops.InodeID(newInode)
	op.Entry.Attributes = fuseops.InodeAttributes{
		Mode:  mode,
		Nlink: 1,
		Size:  0,
		Uid:   0,
		Gid:   0,
		Atime: now,
		Mtime: now,
		Ctime: now,
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
	defer fs.mu.Unlock()

	if op.KillSuidgid {
		fs.setattrWithKillSuidgid = true
	}

	info, ok := fs.inodes[uint64(op.Inode)]
	if !ok {
		return fuse.ENOENT
	}

	if op.Mode != nil {
		info.mode = *op.Mode
		fs.inodes[uint64(op.Inode)] = info
	}

	if op.Size != nil {
		// Handle file truncation
		if *op.Size < uint64(len(fs.fileData)) {
			fs.fileData = fs.fileData[:*op.Size]
		} else if *op.Size > uint64(len(fs.fileData)) {
			newData := make([]byte, *op.Size)
			copy(newData, fs.fileData)
			fs.fileData = newData
		}
	}

	// Re-fetch to ensure we return the updated attributes
	info = fs.inodes[uint64(op.Inode)]

	size := uint64(0)
	if info.mode.IsRegular() {
		size = uint64(len(fs.fileData))
	}

	now := time.Now()
	op.Attributes = fuseops.InodeAttributes{
		Mode:  info.mode,
		Nlink: 1,
		Size:  size,
		Uid:   0,
		Gid:   0,
		Atime: now,
		Mtime: now,
		Ctime: now,
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
