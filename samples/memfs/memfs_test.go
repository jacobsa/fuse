// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs_test

import (
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
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
// Helpers
////////////////////////////////////////////////////////////////////////

func currentUid() uint32 {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		panic(err)
	}

	return uint32(uid)
}

func currentGid() uint32 {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		panic(err)
	}

	return uint32(gid)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}

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

func (t *MemFSTest) Mkdir_OneLevel() {
	var err error
	var fi os.FileInfo
	var stat *syscall.Stat_t

	dirName := path.Join(t.mfs.Dir(), "dir")

	// Create a directory within the root.
	createTime := t.clock.Now()
	err = os.Mkdir(dirName, 0754)
	AssertEq(nil, err)

	// Simulate time proceeding.
	t.clock.AdvanceTime(time.Second)

	// Stat the directory.
	fi, err = os.Stat(dirName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(os.ModeDir|0754, fi.Mode())
	ExpectEq(0, fi.ModTime().Sub(createTime))
	ExpectTrue(fi.IsDir())

	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(0, stat.Size)
	ExpectEq(createTime, timespecToTime(stat.Atimespec))
	ExpectEq(createTime, timespecToTime(stat.Mtimespec))
	ExpectEq(createTime, timespecToTime(stat.Ctimespec))

	// Read the directory.
	entries, err := ioutil.ReadDir(dirName)

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())
}

func (t *MemFSTest) Mkdir_TwoLevels() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) Mkdir_AlreadyExists() {
	var err error
	dirName := path.Join(t.mfs.Dir(), "dir")

	// Create the directory once.
	err = os.Mkdir(dirName, 0754)
	AssertEq(nil, err)

	// Attempt to create it again.
	err = os.Mkdir(dirName, 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("exists")))
}

func (t *MemFSTest) Mkdir_IntermediateIsFile() {
	var err error

	// Create a file.
	fileName := path.Join(t.mfs.Dir(), "foo")
	err = ioutil.WriteFile(fileName, []byte{}, 0700)
	AssertEq(nil, err)

	// Attempt to create a directory within the file.
	dirName := path.Join(fileName, "dir")
	err = os.Mkdir(dirName, 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("TODO")))
}

func (t *MemFSTest) Mkdir_IntermediateIsNonExistent() {
	var err error

	// Attempt to create a sub-directory of a non-existent sub-directory.
	dirName := path.Join(t.mfs.Dir(), "foo/dir")
	err = os.Mkdir(dirName, 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file or directory")))
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
