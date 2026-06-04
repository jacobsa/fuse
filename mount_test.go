package fuse_test

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

////////////////////////////////////////////////////////////////////////
// minimalFS
////////////////////////////////////////////////////////////////////////

// A minimal fuseutil.FileSystem that can successfully mount but do nothing
// else.
type minimalFS struct {
	fuseutil.NotImplementedFileSystem
}

func (fs *minimalFS) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func TestSuccessfulMount(t *testing.T) {
	ctx := context.Background()

	// Set up a temporary directory.
	dir, err := ioutil.TempDir("", "mount_test")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}

	defer os.RemoveAll(dir)

	// Mount.
	fs := &minimalFS{}
	mfs, err := fuse.Mount(
		dir,
		fuseutil.NewFileSystemServer(fs),
		&fuse.MountConfig{})

	if err != nil {
		t.Fatalf("fuse.Mount: %v", err)
	}

	defer func() {
		if err := mfs.Join(ctx); err != nil {
			t.Errorf("Joining: %v", err)
		}
	}()

	defer fuse.Unmount(mfs.Dir())
}

func TestNonexistentMountPoint(t *testing.T) {
	ctx := context.Background()

	// Set up a temporary directory.
	dir, err := ioutil.TempDir("", "mount_test")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}

	defer os.RemoveAll(dir)

	// Attempt to mount into a sub-directory that doesn't exist.
	fs := &minimalFS{}
	mfs, err := fuse.Mount(
		path.Join(dir, "foo"),
		fuseutil.NewFileSystemServer(fs),
		&fuse.MountConfig{})

	if err == nil {
		fuse.Unmount(mfs.Dir())
		mfs.Join(ctx)
		t.Fatal("fuse.Mount returned nil")
	}

	const want = "no such file"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("Unexpected error: %v", got)
	}
}

////////////////////////////////////////////////////////////////////////
// blockingFS
////////////////////////////////////////////////////////////////////////

type blockingFS struct {
	fuseutil.NotImplementedFileSystem
	enteredCh chan string
	releaseCh chan struct{}
}

func (fs *blockingFS) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp) error {
	return nil
}

func (fs *blockingFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp) error {
	if op.Inode == fuseops.RootInodeID {
		op.Attributes = fuseops.InodeAttributes{
			Mode: 0777 | os.ModeDir,
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *blockingFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp) error {
	fs.enteredCh <- op.Name

	// Block until released.
	<-fs.releaseCh

	if op.Name == "foo" || op.Name == "bar" || op.Name == "baz" {
		op.Entry = fuseops.ChildInodeEntry{
			Child: 100, // Canned ID
			Attributes: fuseops.InodeAttributes{
				Mode: 0444,
			},
		}
		return nil
	}

	return fuse.ENOENT
}

func TestMaxThreads(t *testing.T) {
	ctx := context.Background()

	// Set up a temporary directory.
	dir, err := os.MkdirTemp("", "mount_test")
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	// Mount with MaxThreads = 2.
	fs := &blockingFS{
		enteredCh: make(chan string, 3),
		releaseCh: make(chan struct{}),
	}
	mfs, err := fuse.Mount(
		dir,
		fuseutil.NewFileSystemServer(fs),
		&fuse.MountConfig{
			MaxThreads:           2,
			EnableParallelDirOps: true,
		})
	if err != nil {
		t.Fatalf("fuse.Mount: %v", err)
	}
	defer func() {
		if err := mfs.Join(ctx); err != nil {
			t.Errorf("Joining: %v", err)
		}
	}()
	defer fuse.Unmount(mfs.Dir())

	// Start 3 goroutines, each doing a path lookup.
	errChan := make(chan error, 3)

	go func() {
		_, err := os.Stat(path.Join(dir, "foo"))
		errChan <- err
	}()
	go func() {
		_, err := os.Stat(path.Join(dir, "bar"))
		errChan <- err
	}()
	go func() {
		_, err := os.Stat(path.Join(dir, "baz"))
		errChan <- err
	}()

	// Wait a bit to let all 3 requests be sent by the OS to FUSE.
	// Since we set MaxThreads to 2, the third request should be blocked in the server
	// and never reach LookUpInode.
	time.Sleep(100 * time.Millisecond)

	if got := len(fs.enteredCh); got != 2 {
		t.Errorf("enteredOps was %d, expected 2", got)
	}

	// Release one operation.
	fs.releaseCh <- struct{}{}

	// Give the 3rd operation time to enter.
	time.Sleep(100 * time.Millisecond)

	if got := len(fs.enteredCh); got != 3 {
		t.Errorf("enteredOps after one release was %d, expected 3", got)
	}

	// Release the other two.
	fs.releaseCh <- struct{}{}
	fs.releaseCh <- struct{}{}

	// Wait for all 3 goroutines to finish.
	for i := 0; i < 3; i++ {
		// Note: os.Stat might fail with ENOENT but should not return other errors.
		err := <-errChan
		if err != nil && !os.IsNotExist(err) {
			t.Errorf("os.Stat failed: %v", err)
		}
	}
}
