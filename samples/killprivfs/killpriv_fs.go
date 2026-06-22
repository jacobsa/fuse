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
	inodes                 map[uint64]*inodeInfo
	nextInode              uint64
}

type inodeInfo struct {
	mode     os.FileMode
	parent   uint64
	name     string
	children map[string]uint64
	data     []byte // Per-inode data storage
}

func NewKillPrivFS() *KillPrivFS {
	fs := &KillPrivFS{
		inodes:    make(map[uint64]*inodeInfo),
		nextInode: 2, // Start after root (inode 1)
	}
	fs.inodes[1] = &inodeInfo{
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

	fs.inodes[inodeID] = &inodeInfo{
		mode:   mode,
		parent: 1,
		name:   name,
	}

	fs.inodes[1].children[name] = inodeID
	return fuseops.InodeID(inodeID)
}

// AddTestDir bypasses normal FUSE operations to create test directories with specific mode bits.
func (fs *KillPrivFS) AddTestDir(name string, mode os.FileMode) fuseops.InodeID {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	inodeID := fs.nextInode
	fs.nextInode++

	fs.inodes[inodeID] = &inodeInfo{
		mode:     mode | os.ModeDir,
		parent:   1,
		name:     name,
		children: make(map[string]uint64),
	}

	fs.inodes[1].children[name] = inodeID
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
		size = uint64(len(info.data))
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
		size = uint64(len(childInfo.data))
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

	fs.inodes[newInode] = &inodeInfo{
		mode:     op.Mode | os.ModeDir,
		parent:   uint64(op.Parent),
		name:     op.Name,
		children: make(map[string]uint64),
	}

	parentInfo.children[op.Name] = newInode

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

	fs.inodes[newInode] = &inodeInfo{
		mode:   mode,
		parent: uint64(op.Parent),
		name:   op.Name,
	}

	parentInfo.children[op.Name] = newInode
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

	info, ok := fs.inodes[uint64(op.Inode)]
	if !ok {
		return fuse.ENOENT
	}

	if op.Offset+int64(len(op.Data)) > int64(len(info.data)) {
		newSize := op.Offset + int64(len(op.Data))
		newData := make([]byte, newSize)
		copy(newData, info.data)
		info.data = newData
	}
	copy(info.data[op.Offset:], op.Data)

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
	}

	if op.Size != nil {
		if *op.Size < uint64(len(info.data)) {
			info.data = info.data[:*op.Size]
		} else if *op.Size > uint64(len(info.data)) {
			newData := make([]byte, *op.Size)
			copy(newData, info.data)
			info.data = newData
		}
	}

	size := uint64(0)
	if info.mode.IsRegular() {
		size = uint64(len(info.data))
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

	info, ok := fs.inodes[uint64(op.Inode)]
	if !ok {
		return fuse.ENOENT
	}

	if op.Offset >= int64(len(info.data)) {
		op.BytesRead = 0
		return nil
	}

	n := copy(op.Dst, info.data[op.Offset:])
	op.BytesRead = n
	return nil
}

func (fs *KillPrivFS) ReleaseFileHandle(
	ctx context.Context,
	op *fuseops.ReleaseFileHandleOp) error {
	return nil
}
