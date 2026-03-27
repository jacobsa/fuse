package notify_store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

var timeLen = len(time.Time{}.Format(time.RFC3339))

// NotifyTimer may emit times on Ticks() to trigger filesystem changes. The
// fuse.Server emits the same times in the same order on Tocks(), if not nil, to
// indicate that invalidation is complete.
type NotifyTimer interface {
	Ticks() <-chan time.Time
	Tocks() chan<- time.Time
}

// Create a file system with a single file named 'current_time' which always
// contains the current time.
//
// This filesystem is an analog to the libfuse example here:
// https://github.com/libfuse/libfuse/blob/master/example/notify_store_retrieve.c
//
// Unlike package dynamicfs, this implementation does _not_ depend on direct IO.
// The filesystem directly modifies the page cache so file operations eventually
// observe the changes.
//
// Note that there is overlap with package notify_inval, so that each is a
// self-contained example.
func NewNotifyStoreFS(t NotifyTimer) fuse.Server {
	n := fuse.NewNotifier()
	fs := &notifyStoreFS{
		notifier: n,
		teardown: make(chan struct{}),
	}

	ticks := t.Ticks()
	tocks := t.Tocks()
	go func() {
		for {
			select {
			case t := <-ticks:
				fs.mu.Lock()
				fs.currentTime = t
				fs.mu.Unlock()
				fs.store(t)
				if tocks != nil {
					tocks <- t
				}
			case <-fs.teardown:
				return
			}
		}
	}()

	return fuse.NewServerWithNotifier(n, fuseutil.NewFileSystemServer(fs))
}

type notifyStoreFS struct {
	fuseutil.NotImplementedFileSystem

	notifier *fuse.Notifier
	teardown chan struct{}

	mu          sync.Mutex
	currentTime time.Time
}

const (
	currentTimeFilename = "current_time"

	currentTimeInode = fuseops.RootInodeID + iota
)

func (fs *notifyStoreFS) store(t time.Time) {
	if err := fs.notifier.Store(currentTimeInode, 0, []byte(t.Format(time.RFC3339)+"\n")); err != nil {
		fmt.Printf("error storing current_time inode %v: %v\n", currentTimeInode, err)
	}
}

func (fs *notifyStoreFS) fillStat(ino fuseops.InodeID, attrs *fuseops.InodeAttributes) error {
	switch ino {
	case fuseops.RootInodeID:
		attrs.Nlink = 1
		attrs.Mode = 0555 | os.ModeDir
	case currentTimeInode:
		attrs.Nlink = 1
		attrs.Mode = 0444
		attrs.Size = uint64(timeLen + 1) // with newline
	default:
		return fuse.ENOENT
	}
	return nil
}

func (fs *notifyStoreFS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	if op.Parent != fuseops.RootInodeID {
		return fuse.ENOENT
	}

	switch op.Name {
	case currentTimeFilename:
		op.Entry.Child = currentTimeInode
		fs.fillStat(currentTimeInode, &op.Entry.Attributes)
	default:
		return fuse.ENOENT
	}

	distantFuture := time.Now().Add(time.Hour * 300)
	op.Entry.AttributesExpiration = distantFuture
	op.Entry.EntryExpiration = distantFuture
	return nil
}

func (fs *notifyStoreFS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	return fs.fillStat(op.Inode, &op.Attributes)
}

func (fs *notifyStoreFS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	if op.Inode != fuseops.RootInodeID {
		return fuse.ENOTDIR
	}

	if op.Offset <= 0 {
		op.BytesRead += fuseutil.WriteDirent(op.Dst[op.BytesRead:], fuseutil.Dirent{
			Offset: fuseops.DirOffset(1),
			Inode:  currentTimeInode,
			Name:   currentTimeFilename,
		})
	}
	return nil
}

func (fs *notifyStoreFS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	if op.Inode == fuseops.RootInodeID {
		return syscall.EISDIR
	}
	if op.Inode != currentTimeInode {
		// This should not happen
		return fuse.EIO
	}
	if !op.OpenFlags.IsReadOnly() {
		return syscall.EACCES
	}

	// Make cache persistent even if the file is closed. This makes it easier to
	// see the effects of invalidation.
	op.KeepPageCache = true

	return nil
}

func (fs *notifyStoreFS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	if op.Inode != currentTimeInode {
		return fuse.EIO
	}

	fmt.Print("Direct read received, bypassing page cache")

	fs.mu.Lock()
	t := fs.currentTime
	fs.mu.Unlock()

	contents := t.Format(time.RFC3339) + "\n"

	if op.Offset < int64(len(contents)) {
		op.BytesRead = copy(op.Dst, contents[op.Offset:])
	}
	return nil
}

func (fs *notifyStoreFS) Destroy() {
	close(fs.teardown)
}
