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

func (t *cachingFSTest) openFiles() (foo, dir, bar *os.File) {
	var err error

	foo, err = os.Open(path.Join(t.dir, "foo"))
	AssertEq(nil, err)

	dir, err = os.Open(path.Join(t.dir, "dir"))
	AssertEq(nil, err)

	bar, err = os.Open(path.Join(t.dir, "dir/bar"))
	AssertEq(nil, err)

	return
}

func (t *cachingFSTest) statFiles(
	f, g, h *os.File) (foo, dir, bar os.FileInfo) {
	var err error

	foo, err = f.Stat()
	AssertEq(nil, err)

	dir, err = g.Stat()
	AssertEq(nil, err)

	bar, err = h.Stat()
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
	fooBefore, dirBefore, barBefore := t.statAll()
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
	t.statAll()
	t.fs.RenumberInodes()
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new inode IDs, because the entries should not have been
	// cached.
	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))
}

func (t *NoCachingTest) StatMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	t.statAll()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new mtimes, because the attributes should not have been
	// cached.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

func (t *NoCachingTest) StatRenumberMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	t.statAll()
	t.fs.RenumberInodes()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new inode IDs and mtimes, because nothing should have
	// been cached.
	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

////////////////////////////////////////////////////////////////////////
// Entry caching
////////////////////////////////////////////////////////////////////////

type EntryCachingTest struct {
	cachingFSTest
	lookupEntryTimeout time.Duration
}

var _ SetUpInterface = &EntryCachingTest{}

func init() { RegisterTestSuite(&EntryCachingTest{}) }

func (t *EntryCachingTest) SetUp(ti *TestInfo) {
	t.lookupEntryTimeout = 250 * time.Millisecond
	t.cachingFSTest.setUp(t.lookupEntryTimeout, 0)
}

func (t *EntryCachingTest) StatStat() {
	fooBefore, dirBefore, barBefore := t.statAll()
	fooAfter, dirAfter, barAfter := t.statAll()

	// Make sure everything matches.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(fooBefore.ModTime()))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(dirBefore.ModTime()))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(barBefore.ModTime()))

	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))
}

func (t *EntryCachingTest) StatRenumberStat() {
	fooBefore, dirBefore, barBefore := t.statAll()
	t.fs.RenumberInodes()
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should still see the old inode IDs, because the inode entries should
	// have been cached.
	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))

	// But after waiting for the entry cache to expire, we should see the new
	// IDs.
	time.Sleep(2 * t.lookupEntryTimeout)
	fooAfter, dirAfter, barAfter = t.statAll()

	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))
}

func (t *EntryCachingTest) StatMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	t.statAll()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new mtimes, because the attributes should not have been
	// cached.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

func (t *EntryCachingTest) StatRenumberMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	fooBefore, dirBefore, barBefore := t.statAll()
	t.fs.RenumberInodes()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should still see the old inode IDs, because the inode entries should
	// have been cached. But the attributes should not have been.
	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))

	// After waiting for the entry cache to expire, we should see fresh
	// everything.
	time.Sleep(2 * t.lookupEntryTimeout)
	fooAfter, dirAfter, barAfter = t.statAll()

	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

////////////////////////////////////////////////////////////////////////
// Attribute caching
////////////////////////////////////////////////////////////////////////

type AttributeCachingTest struct {
	cachingFSTest
	getattrTimeout time.Duration
}

var _ SetUpInterface = &AttributeCachingTest{}

func init() { RegisterTestSuite(&AttributeCachingTest{}) }

func (t *AttributeCachingTest) SetUp(ti *TestInfo) {
	t.getattrTimeout = 250 * time.Millisecond
	t.cachingFSTest.setUp(0, t.getattrTimeout)
}

func (t *AttributeCachingTest) StatStat() {
	fooBefore, dirBefore, barBefore := t.statAll()
	fooAfter, dirAfter, barAfter := t.statAll()

	// Make sure everything matches.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(fooBefore.ModTime()))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(dirBefore.ModTime()))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(barBefore.ModTime()))

	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))
}

func (t *AttributeCachingTest) StatRenumberStat() {
	t.statAll()
	t.fs.RenumberInodes()
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new inode IDs, because the entries should not have been
	// cached.
	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))
}

func (t *AttributeCachingTest) StatMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	t.statAll()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see the new attributes, since the entry had to be looked up
	// again and the new attributes were returned with the entry.
	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

func (t *AttributeCachingTest) StatRenumberMtimeStat() {
	newMtime := t.initialMtime.Add(time.Second)

	t.statAll()
	t.fs.RenumberInodes()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statAll()

	// We should see new everything, because this is the first time the new
	// inodes have been encountered. Entries for the old ones should not have
	// been cached.
	ExpectEq(t.fs.FooID(), getInodeID(fooAfter))
	ExpectEq(t.fs.DirID(), getInodeID(dirAfter))
	ExpectEq(t.fs.BarID(), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}

func (t *AttributeCachingTest) StatRenumberMtimeStat_ViaFileDescriptor() {
	newMtime := t.initialMtime.Add(time.Second)

	// Open everything, fixing a particular inode number for each.
	foo, dir, bar := t.openFiles()
	defer func() {
		foo.Close()
		dir.Close()
		bar.Close()
	}()

	fooBefore, dirBefore, barBefore := t.statFiles(foo, dir, bar)
	t.fs.RenumberInodes()
	t.fs.SetMtime(newMtime)
	fooAfter, dirAfter, barAfter := t.statFiles(foo, dir, bar)

	// We should still see the old cached mtime with the old inode ID.
	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(fooBefore.ModTime()))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(dirBefore.ModTime()))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(barBefore.ModTime()))

	// After waiting for the attribute cache to expire, we should see the fresh
	// mtime, still with the old inode ID.
	time.Sleep(2 * t.getattrTimeout)
	fooAfter, dirAfter, barAfter = t.statAll()

	ExpectEq(getInodeID(fooBefore), getInodeID(fooAfter))
	ExpectEq(getInodeID(dirBefore), getInodeID(dirAfter))
	ExpectEq(getInodeID(barBefore), getInodeID(barAfter))

	ExpectThat(fooAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(dirAfter.ModTime(), timeutil.TimeEq(newMtime))
	ExpectThat(barAfter.ModTime(), timeutil.TimeEq(newMtime))
}
