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

package memfs_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/jacobsa/fuse/fusetesting"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/memfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
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

// Transform the supplied mode by the current umask.
func applyUmask(m os.FileMode) os.FileMode {
	// HACK(jacobsa): Use umask(2) to change and restore the umask in order to
	// figure out what the mask is. See the listing in `man getumask`.
	umask := syscall.Umask(0)
	syscall.Umask(umask)

	// Apply it.
	return m &^ os.FileMode(umask)
}

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type MemFSTest struct {
	samples.SampleTest
}

func init() { RegisterTestSuite(&MemFSTest{}) }

func (t *MemFSTest) SetUp(ti *TestInfo) {
	t.Server = memfs.NewMemFS(currentUid(), currentGid(), &t.Clock)
	t.SampleTest.SetUp(ti)
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *MemFSTest) ContentsOfEmptyFileSystem() {
	entries, err := fusetesting.ReadDirPicky(t.Dir)

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())
}

func (t *MemFSTest) Mkdir_OneLevel() {
	var err error
	var fi os.FileInfo
	var stat *syscall.Stat_t
	var entries []os.FileInfo

	dirName := path.Join(t.Dir, "dir")

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Create a directory within the root.
	createTime := t.Clock.Now()
	err = os.Mkdir(dirName, 0754)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat the directory.
	fi, err = os.Stat(dirName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(os.ModeDir|applyUmask(0754), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(createTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectTrue(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(0, stat.Size)

	// Check the root's mtime.
	fi, err = os.Stat(t.Dir)

	AssertEq(nil, err)
	ExpectEq(0, fi.ModTime().Sub(createTime))

	// Read the directory.
	entries, err = fusetesting.ReadDirPicky(dirName)

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())

	// Read the root.
	entries, err = fusetesting.ReadDirPicky(t.Dir)

	AssertEq(nil, err)
	AssertEq(1, len(entries))

	fi = entries[0]
	ExpectEq("dir", fi.Name())
	ExpectEq(os.ModeDir|applyUmask(0754), fi.Mode())
}

func (t *MemFSTest) Mkdir_TwoLevels() {
	var err error
	var fi os.FileInfo
	var stat *syscall.Stat_t
	var entries []os.FileInfo

	// Create a directory within the root.
	err = os.Mkdir(path.Join(t.Dir, "parent"), 0700)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Create a child of that directory.
	createTime := t.Clock.Now()
	err = os.Mkdir(path.Join(t.Dir, "parent/dir"), 0754)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat the directory.
	fi, err = os.Stat(path.Join(t.Dir, "parent/dir"))
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(os.ModeDir|applyUmask(0754), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(createTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectTrue(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(0, stat.Size)

	// Check the parent's mtime.
	fi, err = os.Stat(path.Join(t.Dir, "parent"))
	AssertEq(nil, err)
	ExpectEq(0, fi.ModTime().Sub(createTime))

	// Read the directory.
	entries, err = fusetesting.ReadDirPicky(path.Join(t.Dir, "parent/dir"))

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())

	// Read the parent.
	entries, err = fusetesting.ReadDirPicky(path.Join(t.Dir, "parent"))

	AssertEq(nil, err)
	AssertEq(1, len(entries))

	fi = entries[0]
	ExpectEq("dir", fi.Name())
	ExpectEq(os.ModeDir|applyUmask(0754), fi.Mode())
}

func (t *MemFSTest) Mkdir_AlreadyExists() {
	var err error
	dirName := path.Join(t.Dir, "dir")

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
	fileName := path.Join(t.Dir, "foo")
	err = ioutil.WriteFile(fileName, []byte{}, 0700)
	AssertEq(nil, err)

	// Attempt to create a directory within the file.
	dirName := path.Join(fileName, "dir")
	err = os.Mkdir(dirName, 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("not a directory")))
}

func (t *MemFSTest) Mkdir_IntermediateIsNonExistent() {
	var err error

	// Attempt to create a sub-directory of a non-existent sub-directory.
	dirName := path.Join(t.Dir, "foo/dir")
	err = os.Mkdir(dirName, 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file or directory")))
}

func (t *MemFSTest) Mkdir_PermissionDenied() {
	var err error

	// Create a directory within the root without write permissions.
	err = os.Mkdir(path.Join(t.Dir, "parent"), 0500)
	AssertEq(nil, err)

	// Attempt to create a child of that directory.
	err = os.Mkdir(path.Join(t.Dir, "parent/dir"), 0754)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("permission denied")))
}

func (t *MemFSTest) CreateNewFile_InRoot() {
	var err error
	var fi os.FileInfo
	var stat *syscall.Stat_t

	// Write a file.
	fileName := path.Join(t.Dir, "foo")
	const contents = "Hello\x00world"

	createTime := t.Clock.Now()
	err = ioutil.WriteFile(fileName, []byte(contents), 0400)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat it.
	fi, err = os.Stat(fileName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("foo", fi.Name())
	ExpectEq(len(contents), fi.Size())
	ExpectEq(applyUmask(0400), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(createTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectFalse(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(len(contents), stat.Size)

	// Read it back.
	slice, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq(contents, string(slice))
}

func (t *MemFSTest) CreateNewFile_InSubDir() {
	var err error
	var fi os.FileInfo
	var stat *syscall.Stat_t

	// Create a sub-dir.
	dirName := path.Join(t.Dir, "dir")
	err = os.Mkdir(dirName, 0700)
	AssertEq(nil, err)

	// Write a file.
	fileName := path.Join(dirName, "foo")
	const contents = "Hello\x00world"

	createTime := t.Clock.Now()
	err = ioutil.WriteFile(fileName, []byte(contents), 0400)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat it.
	fi, err = os.Stat(fileName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("foo", fi.Name())
	ExpectEq(len(contents), fi.Size())
	ExpectEq(applyUmask(0400), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(createTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectFalse(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(len(contents), stat.Size)

	// Read it back.
	slice, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq(contents, string(slice))
}

func (t *MemFSTest) ModifyExistingFile_InRoot() {
	var err error
	var n int
	var fi os.FileInfo
	var stat *syscall.Stat_t

	// Write a file.
	fileName := path.Join(t.Dir, "foo")

	createTime := t.Clock.Now()
	err = ioutil.WriteFile(fileName, []byte("Hello, world!"), 0600)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Open the file and modify it.
	f, err := os.OpenFile(fileName, os.O_WRONLY, 0400)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	modifyTime := t.Clock.Now()
	n, err = f.WriteAt([]byte("H"), 0)
	AssertEq(nil, err)
	AssertEq(1, n)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat the file.
	fi, err = os.Stat(fileName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("foo", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(applyUmask(0600), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(modifyTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectFalse(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(len("Hello, world!"), stat.Size)

	// Read the file back.
	slice, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq("Hello, world!", string(slice))
}

func (t *MemFSTest) ModifyExistingFile_InSubDir() {
	var err error
	var n int
	var fi os.FileInfo
	var stat *syscall.Stat_t

	// Create a sub-directory.
	dirName := path.Join(t.Dir, "dir")
	err = os.Mkdir(dirName, 0700)
	AssertEq(nil, err)

	// Write a file.
	fileName := path.Join(dirName, "foo")

	createTime := t.Clock.Now()
	err = ioutil.WriteFile(fileName, []byte("Hello, world!"), 0600)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Open the file and modify it.
	f, err := os.OpenFile(fileName, os.O_WRONLY, 0400)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	modifyTime := t.Clock.Now()
	n, err = f.WriteAt([]byte("H"), 0)
	AssertEq(nil, err)
	AssertEq(1, n)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Stat the file.
	fi, err = os.Stat(fileName)
	stat = fi.Sys().(*syscall.Stat_t)

	AssertEq(nil, err)
	ExpectEq("foo", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(applyUmask(0600), fi.Mode())
	ExpectThat(fi, fusetesting.MtimeIs(modifyTime))
	ExpectThat(fi, fusetesting.BirthtimeIs(createTime))
	ExpectFalse(fi.IsDir())

	ExpectNe(0, stat.Ino)
	ExpectEq(1, stat.Nlink)
	ExpectEq(currentUid(), stat.Uid)
	ExpectEq(currentGid(), stat.Gid)
	ExpectEq(len("Hello, world!"), stat.Size)

	// Read the file back.
	slice, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq("Hello, world!", string(slice))
}

func (t *MemFSTest) UnlinkFile_Exists() {
	var err error

	// Write a file.
	fileName := path.Join(t.Dir, "foo")
	err = ioutil.WriteFile(fileName, []byte("Hello, world!"), 0600)
	AssertEq(nil, err)

	// Unlink it.
	err = os.Remove(fileName)
	AssertEq(nil, err)

	// Statting it should fail.
	_, err = os.Stat(fileName)

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file")))

	// Nothing should be in the directory.
	entries, err := fusetesting.ReadDirPicky(t.Dir)
	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())
}

func (t *MemFSTest) UnlinkFile_NonExistent() {
	err := os.Remove(path.Join(t.Dir, "foo"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file")))
}

func (t *MemFSTest) UnlinkFile_StillOpen() {
	fileName := path.Join(t.Dir, "foo")

	// Create and open a file.
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0600)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Write some data into it.
	n, err := f.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	// Unlink it.
	err = os.Remove(fileName)
	AssertEq(nil, err)

	// The directory should no longer contain it.
	entries, err := fusetesting.ReadDirPicky(t.Dir)
	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())

	// We should be able to stat the file. It should still show as having
	// contents, but with no links.
	fi, err := f.Stat()

	AssertEq(nil, err)
	ExpectEq(4, fi.Size())
	ExpectEq(0, fi.Sys().(*syscall.Stat_t).Nlink)

	// The contents should still be available.
	buf := make([]byte, 1024)
	n, err = f.ReadAt(buf, 0)

	AssertEq(io.EOF, err)
	AssertEq(4, n)
	ExpectEq("taco", string(buf[:4]))

	// Writing should still work, too.
	n, err = f.Write([]byte("burrito"))
	AssertEq(nil, err)
	AssertEq(len("burrito"), n)
}

func (t *MemFSTest) Rmdir_NonEmpty() {
	var err error

	// Create two levels of directories.
	err = os.MkdirAll(path.Join(t.Dir, "foo/bar"), 0754)
	AssertEq(nil, err)

	// Attempt to remove the parent.
	err = os.Remove(path.Join(t.Dir, "foo"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("not empty")))
}

func (t *MemFSTest) Rmdir_Empty() {
	var err error
	var entries []os.FileInfo

	// Create two levels of directories.
	err = os.MkdirAll(path.Join(t.Dir, "foo/bar"), 0754)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Remove the leaf.
	rmTime := t.Clock.Now()
	err = os.Remove(path.Join(t.Dir, "foo/bar"))
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// There should be nothing left in the parent.
	entries, err = fusetesting.ReadDirPicky(path.Join(t.Dir, "foo"))

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())

	// Check the parent's mtime.
	fi, err := os.Stat(path.Join(t.Dir, "foo"))
	AssertEq(nil, err)
	ExpectEq(0, fi.ModTime().Sub(rmTime))

	// Remove the parent.
	err = os.Remove(path.Join(t.Dir, "foo"))
	AssertEq(nil, err)

	// Now the root directory should be empty, too.
	entries, err = fusetesting.ReadDirPicky(t.Dir)

	AssertEq(nil, err)
	ExpectThat(entries, ElementsAre())
}

func (t *MemFSTest) Rmdir_NonExistent() {
	err := os.Remove(path.Join(t.Dir, "blah"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file or directory")))
}

func (t *MemFSTest) Rmdir_OpenedForReading() {
	var err error

	// Create a directory.
	createTime := t.Clock.Now()
	err = os.Mkdir(path.Join(t.Dir, "dir"), 0700)
	AssertEq(nil, err)

	// Simulate time advancing.
	t.Clock.AdvanceTime(time.Second)

	// Open the directory for reading.
	f, err := os.Open(path.Join(t.Dir, "dir"))
	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	AssertEq(nil, err)

	// Remove the directory.
	err = os.Remove(path.Join(t.Dir, "dir"))
	AssertEq(nil, err)

	// Create a new directory, with the same name even, and add some contents
	// within it.
	err = os.MkdirAll(path.Join(t.Dir, "dir/foo"), 0700)
	AssertEq(nil, err)

	err = os.MkdirAll(path.Join(t.Dir, "dir/bar"), 0700)
	AssertEq(nil, err)

	err = os.MkdirAll(path.Join(t.Dir, "dir/baz"), 0700)
	AssertEq(nil, err)

	// We should still be able to stat the open file handle. It should show up as
	// unlinked.
	fi, err := f.Stat()

	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.ModTime().Sub(createTime))
	ExpectEq(0, fi.Sys().(*syscall.Stat_t).Nlink)

	// Attempt to read from the directory. This shouldn't see any junk from the
	// new directory. It should either succeed with an empty result or should
	// return ENOENT.
	names, err := f.Readdirnames(0)

	if err != nil {
		ExpectThat(err, Error(HasSubstr("no such file")))
	} else {
		ExpectThat(names, ElementsAre())
	}
}

func (t *MemFSTest) CaseSensitive() {
	var err error

	// Create a file.
	err = ioutil.WriteFile(path.Join(t.Dir, "file"), []byte{}, 0400)
	AssertEq(nil, err)

	// Create a directory.
	err = os.Mkdir(path.Join(t.Dir, "dir"), 0400)
	AssertEq(nil, err)

	// Attempt to stat with the wrong case.
	names := []string{
		"FILE",
		"File",
		"filE",
		"DIR",
		"Dir",
		"dIr",
	}

	for _, name := range names {
		_, err = os.Stat(path.Join(t.Dir, name))
		AssertNe(nil, err, "Name: %s", name)
		AssertThat(err, Error(HasSubstr("no such file or directory")))
	}
}

func (t *MemFSTest) WriteOverlapsEndOfFile() {
	var err error
	var n int

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Make it 4 bytes long.
	err = f.Truncate(4)
	AssertEq(nil, err)

	// Write the range [2, 6).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// Read the full contents of the file.
	contents, err := ioutil.ReadAll(f)
	AssertEq(nil, err)
	ExpectEq("\x00\x00taco", string(contents))
}

func (t *MemFSTest) WriteStartsAtEndOfFile() {
	var err error
	var n int

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Make it 2 bytes long.
	err = f.Truncate(2)
	AssertEq(nil, err)

	// Write the range [2, 6).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// Read the full contents of the file.
	contents, err := ioutil.ReadAll(f)
	AssertEq(nil, err)
	ExpectEq("\x00\x00taco", string(contents))
}

func (t *MemFSTest) WriteStartsPastEndOfFile() {
	var err error
	var n int

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Write the range [2, 6).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// Read the full contents of the file.
	contents, err := ioutil.ReadAll(f)
	AssertEq(nil, err)
	ExpectEq("\x00\x00taco", string(contents))
}

func (t *MemFSTest) WriteAtDoesntChangeOffset_NotAppendMode() {
	var err error
	var n int

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Make it 16 bytes long.
	err = f.Truncate(16)
	AssertEq(nil, err)

	// Seek to offset 4.
	_, err = f.Seek(4, 0)
	AssertEq(nil, err)

	// Write the range [10, 14).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// We should still be at offset 4.
	offset, err := getFileOffset(f)
	AssertEq(nil, err)
	ExpectEq(4, offset)
}

func (t *MemFSTest) WriteAtDoesntChangeOffset_AppendMode() {
	var err error
	var n int

	// Create a file in append mode.
	f, err := os.OpenFile(
		path.Join(t.Dir, "foo"),
		os.O_RDWR|os.O_APPEND|os.O_CREATE,
		0600)

	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Make it 16 bytes long.
	err = f.Truncate(16)
	AssertEq(nil, err)

	// Seek to offset 4.
	_, err = f.Seek(4, 0)
	AssertEq(nil, err)

	// Write the range [10, 14).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// We should still be at offset 4.
	offset, err := getFileOffset(f)
	AssertEq(nil, err)
	ExpectEq(4, offset)
}

func (t *MemFSTest) LargeFile() {
	var err error

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Copy in large contents.
	const size = 1 << 24
	contents := bytes.Repeat([]byte{0x20}, size)

	_, err = io.Copy(f, bytes.NewReader(contents))
	AssertEq(nil, err)

	// Read the full contents of the file.
	contents, err = ioutil.ReadFile(f.Name())
	AssertEq(nil, err)
	ExpectEq(size, len(contents))
}

func (t *MemFSTest) AppendMode() {
	var err error
	var n int
	var off int64
	buf := make([]byte, 1024)

	// Create a file with some contents.
	fileName := path.Join(t.Dir, "foo")
	err = ioutil.WriteFile(fileName, []byte("Jello, "), 0600)
	AssertEq(nil, err)

	// Open the file in append mode.
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND, 0600)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Seek to somewhere silly and then write.
	off, err = f.Seek(2, 0)
	AssertEq(nil, err)
	AssertEq(2, off)

	n, err = f.Write([]byte("world!"))
	AssertEq(nil, err)
	AssertEq(6, n)

	// The offset should have been updated to point at the end of the file.
	off, err = getFileOffset(f)
	AssertEq(nil, err)
	ExpectEq(13, off)

	// A random write should still work, without updating the offset.
	n, err = f.WriteAt([]byte("H"), 0)
	AssertEq(nil, err)
	AssertEq(1, n)

	off, err = getFileOffset(f)
	AssertEq(nil, err)
	ExpectEq(13, off)

	// Read back the contents of the file, which should be correct even though we
	// seeked to a silly place before writing the world part.
	//
	// Linux's support for pwrite is buggy; the pwrite(2) man page says this:
	//
	//     POSIX requires that opening a file with the O_APPEND flag should have
	//     no affect on the location at which pwrite() writes data.  However, on
	//     Linux,  if  a  file  is opened with O_APPEND, pwrite() appends data to
	//     the end of the file, regardless of the value of offset.
	//
	// So we allow either the POSIX result or the Linux result.
	n, err = f.ReadAt(buf, 0)
	AssertEq(io.EOF, err)
	ExpectThat(string(buf[:n]), AnyOf("Hello, world!", "Jello, world!H"))
}

func (t *MemFSTest) ReadsPastEndOfFile() {
	var err error
	var n int
	buf := make([]byte, 1024)

	// Create a file.
	f, err := os.Create(path.Join(t.Dir, "foo"))
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Give it some contents.
	n, err = f.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	// Read a range overlapping EOF.
	n, err = f.ReadAt(buf[:4], 2)
	AssertEq(io.EOF, err)
	ExpectEq(2, n)
	ExpectEq("co", string(buf[:n]))

	// Read a range starting at EOF.
	n, err = f.ReadAt(buf[:4], 4)
	AssertEq(io.EOF, err)
	ExpectEq(0, n)
	ExpectEq("", string(buf[:n]))

	// Read a range starting past EOF.
	n, err = f.ReadAt(buf[:4], 100)
	AssertEq(io.EOF, err)
	ExpectEq(0, n)
	ExpectEq("", string(buf[:n]))
}

func (t *MemFSTest) Truncate_Smaller() {
	var err error
	fileName := path.Join(t.Dir, "foo")

	// Create a file.
	err = ioutil.WriteFile(fileName, []byte("taco"), 0600)
	AssertEq(nil, err)

	// Open it for modification.
	f, err := os.OpenFile(fileName, os.O_RDWR, 0)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Truncate it.
	err = f.Truncate(2)
	AssertEq(nil, err)

	// Stat it.
	fi, err := f.Stat()
	AssertEq(nil, err)
	ExpectEq(2, fi.Size())

	// Read the contents.
	contents, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq("ta", string(contents))
}

func (t *MemFSTest) Truncate_SameSize() {
	var err error
	fileName := path.Join(t.Dir, "foo")

	// Create a file.
	err = ioutil.WriteFile(fileName, []byte("taco"), 0600)
	AssertEq(nil, err)

	// Open it for modification.
	f, err := os.OpenFile(fileName, os.O_RDWR, 0)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Truncate it.
	err = f.Truncate(4)
	AssertEq(nil, err)

	// Stat it.
	fi, err := f.Stat()
	AssertEq(nil, err)
	ExpectEq(4, fi.Size())

	// Read the contents.
	contents, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq("taco", string(contents))
}

func (t *MemFSTest) Truncate_Larger() {
	var err error
	fileName := path.Join(t.Dir, "foo")

	// Create a file.
	err = ioutil.WriteFile(fileName, []byte("taco"), 0600)
	AssertEq(nil, err)

	// Open it for modification.
	f, err := os.OpenFile(fileName, os.O_RDWR, 0)
	t.ToClose = append(t.ToClose, f)
	AssertEq(nil, err)

	// Truncate it.
	err = f.Truncate(6)
	AssertEq(nil, err)

	// Stat it.
	fi, err := f.Stat()
	AssertEq(nil, err)
	ExpectEq(6, fi.Size())

	// Read the contents.
	contents, err := ioutil.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq("taco\x00\x00", string(contents))
}

func (t *MemFSTest) Chmod() {
	var err error
	fileName := path.Join(t.Dir, "foo")

	// Create a file.
	err = ioutil.WriteFile(fileName, []byte(""), 0600)
	AssertEq(nil, err)

	// Chmod it.
	err = os.Chmod(fileName, 0754)
	AssertEq(nil, err)

	// Stat it.
	fi, err := os.Stat(fileName)
	AssertEq(nil, err)
	ExpectEq(0754, fi.Mode())
}

func (t *MemFSTest) Chtimes() {
	var err error
	fileName := path.Join(t.Dir, "foo")

	// Create a file.
	err = ioutil.WriteFile(fileName, []byte(""), 0600)
	AssertEq(nil, err)

	// Chtimes it.
	expectedMtime := time.Now().Add(123 * time.Second).Round(time.Second)
	err = os.Chtimes(fileName, time.Now(), expectedMtime)
	AssertEq(nil, err)

	// Stat it.
	fi, err := os.Stat(fileName)
	AssertEq(nil, err)
	ExpectEq(0, fi.ModTime().Sub(expectedMtime))
}

func (t *MemFSTest) ReadDirWhileModifying() {
	dirName := path.Join(t.Dir, "dir")
	createFile := func(name string) {
		AssertEq(nil, ioutil.WriteFile(path.Join(dirName, name), []byte{}, 0400))
	}

	// Create a directory.
	err := os.Mkdir(dirName, 0700)
	AssertEq(nil, err)

	// Open the directory.
	d, err := os.Open(dirName)
	t.ToClose = append(t.ToClose, d)
	AssertEq(nil, err)

	// Add four files.
	createFile("foo")
	createFile("bar")
	createFile("baz")
	createFile("qux")

	// Read one entry from the directory.
	names, err := d.Readdirnames(1)
	AssertEq(nil, err)
	AssertThat(names, ElementsAre("foo"))

	// Make two holes in the directory.
	AssertEq(nil, os.Remove(path.Join(dirName, "foo")))
	AssertEq(nil, os.Remove(path.Join(dirName, "baz")))

	// Add a bunch of files to the directory.
	createFile("blah_0")
	createFile("blah_1")
	createFile("blah_2")
	createFile("blah_3")
	createFile("blah_4")

	// Continue reading from the directory, noting the names we see.
	namesSeen := make(map[string]bool)
	for {
		names, err = d.Readdirnames(1)
		for _, n := range names {
			namesSeen[n] = true
		}

		if err == io.EOF {
			break
		}

		AssertEq(nil, err)
	}

	// Posix requires that we should have seen bar and qux, which we didn't
	// delete.
	ExpectTrue(namesSeen["bar"])
	ExpectTrue(namesSeen["qux"])
}

func (t *MemFSTest) HardLinks() {
	var err error

	// Create a file and a directory.
	fileName := path.Join(t.Dir, "foo")
	err = ioutil.WriteFile(fileName, []byte{}, 0400)
	AssertEq(nil, err)

	dirName := path.Join(t.Dir, "bar")
	err = os.Mkdir(dirName, 0700)
	AssertEq(nil, err)

	// Attempt to link each. Neither should work, but for different reasons.
	err = os.Link(fileName, path.Join(t.Dir, "baz"))
	ExpectThat(err, Error(HasSubstr("not implemented")))

	err = os.Link(dirName, path.Join(t.Dir, "baz"))
	ExpectThat(err, Error(HasSubstr("not permitted")))
}

func (t *MemFSTest) CreateSymlink() {
	AssertTrue(false, "TODO")
}

func (t *MemFSTest) CreateSymlink_AlreadyExists() {
	AssertTrue(false, "TODO")
}
