package notify_inval

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

// Create a file system with two files:
// One is empty, and its name is the current time.
// The other is named 'current_time' and always contains the current time.
//
// This filesystem is an analog to the libfuse examples here:
// https://github.com/libfuse/libfuse/blob/e75d2c54a347906478724be24bfa1df2638094cb/example/notify_inval_inode.c
// https://github.com/libfuse/libfuse/blob/e75d2c54a347906478724be24bfa1df2638094cb/example/notify_inval_entry.c
//
// Unlike package dynamicfs, this implementation does _not_ depend on direct IO.
// The invalidations allow file operations to eventually observe the changes.
func NewNotifyInvalFS(t NotifyTimer) fuse.Server {
	n := fuse.NewNotifier()
	fs := &notifyInvalInodeFS{
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
				oldtime := fs.currentTime
				fs.currentTime = t
				fs.mu.Unlock()
				fs.invalidateInodes(oldtime)
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

type notifyInvalInodeFS struct {
	fuseutil.NotImplementedFileSystem

	notifier *fuse.Notifier
	teardown chan struct{}

	mu sync.Mutex
	// GUARDED_BY(mu)
	currentTime time.Time
}

const (
	currentTimeFilename = "current_time"

	currentTimeInode = fuseops.RootInodeID + iota
	changingFnameInode
)

func (fs *notifyInvalInodeFS) invalidateInodes(oldTime time.Time) {
	// Invalidate inode cache and dcache for both dynamic files.
	if err := fs.notifier.InvalidateInode(currentTimeInode, 0, 0); err != nil {
		fmt.Printf("error invalidating current_time inode %v: %v\n", currentTimeInode, err)
	}
	if err := fs.notifier.InvalidateEntry(fuseops.RootInodeID, currentTimeFilename); err != nil {
		fmt.Printf("error invalidating current_time entry %v for parent %v: %v\n", currentTimeFilename, fuseops.RootInodeID, err)
	}

	if err := fs.notifier.InvalidateInode(changingFnameInode, 0, 0); err != nil {
		fmt.Printf("error invalidating dynamic filename inode %v: %v\n", changingFnameInode, err)
	}
	if err := fs.notifier.InvalidateEntry(fuseops.RootInodeID, oldTime.Format(time.RFC3339)); err != nil {
		fmt.Printf("error invalidating dynamic filename entry for parent %v: %v\n", fuseops.RootInodeID, err)
	}
}

func (fs *notifyInvalInodeFS) fillStat(ino fuseops.InodeID, attrs *fuseops.InodeAttributes) error {
	switch ino {
	case fuseops.RootInodeID:
		attrs.Nlink = 1
		attrs.Mode = 0555 | os.ModeDir
	case currentTimeInode:
		attrs.Nlink = 1
		attrs.Mode = 0444
		attrs.Size = uint64(timeLen + 1) // with newline
	case changingFnameInode:
		attrs.Nlink = 1
		attrs.Mode = 0444
	default:
		return fuse.ENOENT
	}
	return nil
}

func (fs *notifyInvalInodeFS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	if op.Parent != fuseops.RootInodeID {
		return fuse.ENOENT
	}

	fs.mu.Lock()
	t := fs.currentTime
	fs.mu.Unlock()

	switch op.Name {
	case currentTimeFilename:
		op.Entry.Child = currentTimeInode
		fs.fillStat(currentTimeInode, &op.Entry.Attributes)
	case t.Format(time.RFC3339):
		op.Entry.Child = changingFnameInode
		fs.fillStat(changingFnameInode, &op.Entry.Attributes)
	default:
		return fuse.ENOENT
	}

	distantFuture := time.Now().Add(time.Hour * 300)
	op.Entry.AttributesExpiration = distantFuture
	op.Entry.EntryExpiration = distantFuture
	return nil
}

func (fs *notifyInvalInodeFS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	return fs.fillStat(op.Inode, &op.Attributes)
}

func (fs *notifyInvalInodeFS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	if op.Inode != fuseops.RootInodeID {
		return fuse.ENOTDIR
	}

	fs.mu.Lock()
	t := fs.currentTime
	fs.mu.Unlock()

	if op.Offset <= 0 {
		op.BytesRead += fuseutil.WriteDirent(op.Dst[op.BytesRead:], fuseutil.Dirent{
			Offset: fuseops.DirOffset(1),
			Inode:  currentTimeInode,
			Name:   currentTimeFilename,
		})
	}
	if op.Offset <= 1 {
		op.BytesRead += fuseutil.WriteDirent(op.Dst[op.BytesRead:], fuseutil.Dirent{
			Offset: fuseops.DirOffset(2),
			Inode:  changingFnameInode,
			Name:   t.Format(time.RFC3339),
		})
	}
	return nil
}

func (fs *notifyInvalInodeFS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	if op.Inode == fuseops.RootInodeID {
		return syscall.EISDIR
	}
	if op.Inode == changingFnameInode {
		// No access to the changing filename contents
		return syscall.EACCES
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

func (fs *notifyInvalInodeFS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	if op.Inode != currentTimeInode {
		return fuse.EIO
	}

	fs.mu.Lock()
	t := fs.currentTime
	fs.mu.Unlock()

	contents := t.Format(time.RFC3339) + "\n"

	if op.Offset < int64(len(contents)) {
		op.BytesRead = copy(op.Dst, contents[op.Offset:])
	}
	return nil
}

func (fs *notifyInvalInodeFS) Destroy() {
	close(fs.teardown)
}
