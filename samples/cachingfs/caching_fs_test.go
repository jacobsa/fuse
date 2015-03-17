// Copyright 2015 Google Inc. All Rights Reserved.
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

package cachingfs_test

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/cachingfs"
	"github.com/jacobsa/gcsfuse/timeutil"
	. "github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

func TestHelloFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type cachingFSTest struct {
	dir          string
	fs           cachingfs.CachingFS
	mfs          *fuse.MountedFileSystem
	initialMtime time.Time
}

var _ TearDownInterface = &cachingFSTest{}

func (t *cachingFSTest) setUp(
	lookupEntryTimeout time.Duration,
	getattrTimeout time.Duration) {
	var err error

	// Set up a temporary directory for mounting.
	t.dir, err = ioutil.TempDir("", "caching_fs_test")
	AssertEq(nil, err)

	// Create the file system.
	t.fs, err = cachingfs.NewCachingFS(lookupEntryTimeout, getattrTimeout)
	AssertEq(nil, err)

	// Mount it.
	t.mfs, err = fuse.Mount(t.dir, t.fs)
	AssertEq(nil, err)

	err = t.mfs.WaitForReady(context.Background())
	AssertEq(nil, err)

	// Set up the mtime.
	t.initialMtime = time.Date(2012, 8, 15, 22, 56, 0, 0, time.Local)
	t.fs.SetMtime(t.initialMtime)
}

func (t *cachingFSTest) TearDown() {
	// Was the file system mounted?
	if t.mfs == nil {
		return
	}

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

func (t *cachingFSTest) statAll() (foo, dir, bar os.FileInfo) {
	var err error

	foo, err = os.Stat(path.Join(t.dir, "foo"))
	AssertEq(nil, err)

	dir, err = os.Stat(path.Join(t.dir, "dir"))
	AssertEq(nil, err)

	bar, err = os.Stat(path.Join(t.dir, "dir/bar"))
	AssertEq(nil, err)

	return
}

func getInodeID(fi os.FileInfo) uint64 {
	return fi.Sys().(*syscall.Stat_t).Ino
}

////////////////////////////////////////////////////////////////////////
// Basics
////////////////////////////////////////////////////////////////////////

type BasicsTest struct {
	cachingFSTest
}

var _ SetUpInterface = &BasicsTest{}

func init() { RegisterTestSuite(&BasicsTest{}) }

func (t *BasicsTest) SetUp(ti *TestInfo) {
	const (
		lookupEntryTimeout = 0
		getattrTimeout     = 0
	)

	t.cachingFSTest.setUp(lookupEntryTimeout, getattrTimeout)
}

func (t *BasicsTest) StatNonexistent() {
	names := []string{
		"blah",
		"bar",
		"dir/blah",
		"dir/dir",
		"dir/foo",
	}

	for _, n := range names {
		_, err := os.Stat(path.Join(t.dir, n))

		AssertNe(nil, err)
		ExpectTrue(os.IsNotExist(err), "n: %s, err: %v", n, err)
	}
}

func (t *BasicsTest) StatFoo() {
	fi, err := os.Stat(path.Join(t.dir, "foo"))
	AssertEq(nil, err)

	ExpectEq("foo", fi.Name())
	ExpectEq(cachingfs.FooSize, fi.Size())
	ExpectEq(0777, fi.Mode())
	ExpectThat(fi.ModTime(), timeutil.TimeEq(t.initialMtime))
	ExpectFalse(fi.IsDir())
	ExpectEq(t.fs.FooID(), getInodeID(fi))
}

func (t *BasicsTest) StatDir() {
	fi, err := os.Stat(path.Join(t.dir, "dir"))
	AssertEq(nil, err)

	ExpectEq("dir", fi.Name())
	ExpectEq(os.ModeDir|0777, fi.Mode())
	ExpectThat(fi.ModTime(), timeutil.TimeEq(t.initialMtime))
	ExpectTrue(fi.IsDir())
	ExpectEq(t.fs.DirID(), getInodeID(fi))
}

func (t *BasicsTest) StatBar() {
	fi, err := os.Stat(path.Join(t.dir, "dir/bar"))
	AssertEq(nil, err)

	ExpectEq("bar", fi.Name())
	ExpectEq(cachingfs.BarSize, fi.Size())
	ExpectEq(0777, fi.Mode())
	ExpectThat(fi.ModTime(), timeutil.TimeEq(t.initialMtime))
	ExpectFalse(fi.IsDir())
	ExpectEq(t.fs.BarID(), getInodeID(fi))
}

////////////////////////////////////////////////////////////////////////
// No caching
////////////////////////////////////////////////////////////////////////

type NoCachingTest struct {
	cachingFSTest
}

var _ SetUpInterface = &NoCachingTest{}

func init() { RegisterTestSuite(&NoCachingTest{}) }

func (t *NoCachingTest) SetUp(ti *TestInfo) {
	const (
		lookupEntryTimeout = 0
		getattrTimeout     = 0
	)

	t.cachingFSTest.setUp(lookupEntryTimeout, getattrTimeout)
}

func (t *NoCachingTest) StatStat() {
	// Stat everything.
	fooBefore, dirBefore, barBefore := t.statAll()

	// Stat again.
	fooAfter, dirAfter, barAfter := t.statAll()

	// Make sure everything matches.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(fooBefore.ModTime()))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(dirBefore.ModTime()))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(barBefore.ModTime()))

	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))
}

func (t *NoCachingTest) StatRenumberStat() {
	AssertTrue(false, "TODO")
}

func (t *NoCachingTest) StatMtimeStat() {
	AssertTrue(false, "TODO")
}

func (t *NoCachingTest) StatRenumberMtimeStat() {
	AssertTrue(false, "TODO")
}
