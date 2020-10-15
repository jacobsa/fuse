package dynamicfs

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/timeutil"
)

// Create a file system that contains 2 files (`age` and `weekday`) and no
// directories. Every time the `age` file is opened, its contents are refreshed
// to show the number of seconds elapsed since the file system was created (as
// opposed to mounted). Every time the `weekday` file is opened, its contents
// are refreshed to reflect the current weekday.
//
// The contents of both of these files is updated within the filesystem itself,
// i.e., these changes do not go through the kernel. Additionally, file access
// times are not updated and file size is not known in advance and is set to 0.
// This simulates a filesystem that is backed by a dynamic data source where
// file metadata is not necessarily known before the file is read. For example,
// a filesystem backed by an expensive RPC or by a stream that's generated on
// the fly might not know data size ahead of time.
//
// This implementation depends on direct IO in fuse. Without it, all read
// operations are suppressed because the kernel detects that they read beyond
// the end of the files.
func NewDynamicFS(clock timeutil.Clock) (fuse.Server, error) {
	createTime := clock.Now()
	fs := &dynamicFS{
		clock:       clock,
		createTime:  createTime,
		fileHandles: make(map[fuseops.HandleID]string),
	}
	return fuseutil.NewFileSystemServer(fs), nil
}

type dynamicFS struct {
	fuseutil.NotImplementedFileSystem
	mu          sync.Mutex
	clock       timeutil.Clock
	createTime  time.Time
	nextHandle  fuseops.HandleID
	fileHandles map[fuseops.HandleID]string
}

const (
	rootInode fuseops.InodeID = fuseops.RootInodeID + iota
	ageInode
	weekdayInode
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
	rootInode: {
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		},
		dir: true,
		children: []fuseutil.Dirent{
			{
				Offset: 1,
				Inode:  ageInode,
				Name:   "age",
				Type:   fuseutil.DT_File,
			},
			{
				Offset: 2,
				Inode:  weekdayInode,
				Name:   "weekday",
				Type:   fuseutil.DT_File,
			},
		},
	},

	// age
	ageInode: {
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
		},
	},

	// weekday
	weekdayInode: {
		attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			// Size left at 0.
		},
	},
}

func findChildInode(
	name string,
	children []fuseutil.Dirent) (fuseops.InodeID, error) {
	for _, e := range children {
		if e.Name == name {
			return e.Inode, nil
		}
	}

	return 0, fuse.ENOENT
}

func (fs *dynamicFS) findUnusedHandle() fuseops.HandleID {
	// TODO: Mutex annotation?
	handle := fs.nextHandle
	for _, exists := fs.fileHandles[handle]; exists; _, exists = fs.fileHandles[handle] {
		handle++
	}
	fs.nextHandle = handle + 1
	return handle
}

func (fs *dynamicFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	// Find the info for this inode.
	info, ok := gInodeInfo[op.Inode]
	if !ok {
		return fuse.ENOENT
	}
	// Copy over its attributes.
	op.Attributes = info.attributes
	return nil
}

func (fs *dynamicFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	// Find the info for the parent.
	parentInfo, ok := gInodeInfo[op.Parent]
	if !ok {
		return fuse.ENOENT
	}

	// Find the child within the parent.
	childInode, err := findChildInode(op.Name, parentInfo.children)
	if err != nil {
		return err
	}

	// Copy over information.
	op.Entry.Child = childInode
	op.Entry.Attributes = gInodeInfo[childInode].attributes

	return nil
}

func (fs *dynamicFS) OpenDir(
	ctx context.Context,
	op *fuseops.OpenDirOp) error {
	// Allow opening directory.
	return nil
}

func (fs *dynamicFS) ReadDir(
	ctx context.Context,
	op *fuseops.ReadDirOp) error {
	// Find the info for this inode.
	info, ok := gInodeInfo[op.Inode]
	if !ok {
		return fuse.ENOENT
	}

	if !info.dir {
		return fuse.EIO
	}

	entries := info.children

	// Grab the range of interest.
	if op.Offset > fuseops.DirOffset(len(entries)) {
		return fuse.EIO
	}

	entries = entries[op.Offset:]

	// Resume at the specified offset into the array.
	for _, e := range entries {
		n := fuseutil.WriteDirent(op.Dst[op.BytesRead:], e)
		if n == 0 {
			break
		}

		op.BytesRead += n
	}

	return nil
}

func (fs *dynamicFS) OpenFile(
	ctx context.Context,
	op *fuseops.OpenFileOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var contents string
	// Update file contents on (and only on) open.
	switch op.Inode {
	case ageInode:
		now := fs.clock.Now()
		ageInSeconds := int(now.Sub(fs.createTime).Seconds())
		contents = fmt.Sprintf("This filesystem is %d seconds old.", ageInSeconds)
	case weekdayInode:
		contents = fmt.Sprintf("Today is %s.", fs.clock.Now().Weekday())
	default:
		return fuse.EINVAL
	}
	handle := fs.findUnusedHandle()
	fs.fileHandles[handle] = contents
	op.UseDirectIO = true
	op.Handle = handle
	return nil
}

func (fs *dynamicFS) ReadFile(
	ctx context.Context,
	op *fuseops.ReadFileOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	contents, ok := fs.fileHandles[op.Handle]
	if !ok {
		log.Printf("ReadFile: no open file handle: %d", op.Handle)
		return fuse.EIO
	}
	reader := strings.NewReader(contents)
	var err error
	op.BytesRead, err = reader.ReadAt(op.Dst, op.Offset)
	if err == io.EOF {
		return nil
	}
	return err
}

func (fs *dynamicFS) ReleaseFileHandle(
	ctx context.Context,
	op *fuseops.ReleaseFileHandleOp) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	_, ok := fs.fileHandles[op.Handle]
	if !ok {
		log.Printf("ReleaseFileHandle: bad handle: %d", op.Handle)
		return fuse.EIO
	}
	delete(fs.fileHandles, op.Handle)
	return nil
}

func (fs *dynamicFS) StatFS(ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}
