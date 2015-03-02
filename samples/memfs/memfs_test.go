// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs_test

import (
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/memfs"
	"github.com/jacobsa/gcsfuse/timeutil"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

func TestMemFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type MemFSTest struct {
	clock timeutil.SimulatedClock
	mfs   *fuse.MountedFileSystem
}

var _ SetUpInterface = &MemFSTest{}
var _ TearDownInterface = &MemFSTest{}

func init() { RegisterTestSuite(&MemFSTest{}) }

func (t *MemFSTest) SetUp(ti *TestInfo) {
	var err error

	// Set up a fixed, non-zero time.
	t.clock.SetTime(time.Now())

	// Set up a temporary directory for mounting.
	mountPoint, err := ioutil.TempDir("", "memfs_test")
	if err != nil {
		panic("ioutil.TempDir: " + err.Error())
	}

	// Mount a file system.
	fs := memfs.NewMemFS(&t.clock)
	if t.mfs, err = fuse.Mount(mountPoint, fs); err != nil {
		panic("Mount: " + err.Error())
	}

	if err = t.mfs.WaitForReady(context.Background()); err != nil {
		panic("MountedFileSystem.WaitForReady: " + err.Error())
	}
}

func (t *MemFSTest) TearDown() {
	// Unmount the file system. Try again on "resource busy" errors.
	delay := 10 * time.Millisecond
	for {
		err := t.mfs.Unmount()
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "resource busy") {
			log.Println("Resource busy error while unmounting; trying again")
			time.Sleep(delay)
			delay = time.Duration(1.3 * float64(delay))
			continue
		}

		panic("MountedFileSystem.Unmount: " + err.Error())
	}

	if err := t.mfs.Join(context.Background()); err != nil {
		panic("MountedFileSystem.Join: " + err.Error())
	}
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *MemFSTest) ContentsOfEmptyFileSystem() {
	entries, err := ioutil.ReadDir(t.mfs.Dir())

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())
}

func (t *MemFSTest) Mkdir() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Mkdir_AlreadyExists() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) CreateNewFile_InRoot() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) CreateNewFile_InSubDir() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) ModifyExistingFile_InRoot() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) ModifyExistingFile_InSubDir() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) UnlinkFile_Exists() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) UnlinkFile_NotAFile() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) UnlinkFile_NonExistent() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Rmdir_NonEmpty() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Rmdir_Empty() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Rmdir_NotADirectory() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Rmdir_NonExistent() {
	AssertTrue(false, "TODO")
}
