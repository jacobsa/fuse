// Copyright 2025 Google Inc. All Rights Reserved.
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

package wirelog

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	. "github.com/jacobsa/ogletest"
)

func TestWireLog(t *testing.T) { RunTests(t) }

type WireLogTest struct {
	ctx context.Context
	dir string
	mfs *fuse.MountedFileSystem
	buf bytes.Buffer
}

func init() { RegisterTestSuite(&WireLogTest{}) }

func (t *WireLogTest) SetUp(ti *TestInfo) {
	t.ctx = ti.Ctx
	var err error
	t.dir, err = os.MkdirTemp("", "wirelog_test")
	AssertEq(nil, err)

	// Mount the file system.
	t.mfs, err = fuse.Mount(t.dir, NewTestFS(), &fuse.MountConfig{
		WireLogger: &t.buf,
		OpContext:  t.ctx,
	})
	AssertEq(nil, err)
}

func (t *WireLogTest) TearDown() {
	// Ensure unmounted.
	if t.mfs != nil {
		fuse.Unmount(t.dir)
		t.mfs.Join(t.ctx)
	}
	os.RemoveAll(t.dir)
}

// Helper to load Args into a struct
func loadArgs(entry fuse.WireLogRecord, dst any) {
	b, err := json.Marshal(entry.Args)
	AssertEq(nil, err)
	err = json.Unmarshal(b, dst)
	AssertEq(nil, err)
}

func (t *WireLogTest) RunWorkloadAndCheckLogs() {
	// 1. Stat the file.
	filePath := path.Join(t.dir, "foo")
	fi, err := os.Stat(filePath)
	AssertEq(nil, err)
	ExpectEq(3, fi.Size())

	// 2. Read the file.
	content, err := os.ReadFile(filePath)
	AssertEq(nil, err)
	ExpectEq("bar", string(content))

	// Unmount to ensure everything is flushed/closed.
	err = fuse.Unmount(t.dir)
	AssertEq(nil, err)

	// Wait for the connection to close.
	err = t.mfs.Join(t.ctx)
	AssertEq(nil, err)

	// Mark as joined so TearDown doesn't try again.
	t.mfs = nil

	// Parse the logs.
	ops := make(map[string][]fuse.WireLogRecord)
	decoder := json.NewDecoder(&t.buf)

	for decoder.More() {
		var entry fuse.WireLogRecord
		err := decoder.Decode(&entry)
		AssertEq(nil, err)
		ExpectTrue(time.Now().After(entry.StartTime))
		ExpectGt(entry.Duration, 0)
		ops[entry.Operation] = append(ops[entry.Operation], entry)
	}

	// 1. initOp
	entries, ok := ops["initOp"]
	AssertTrue(ok)
	AssertEq(1, len(entries))
	entry := entries[0]
	AssertEq(0, entry.Status)
	AssertEq(nil, entry.Context)

	// 2. LookUpInodeOp
	entries, ok = ops["LookUpInodeOp"]
	AssertTrue(ok)
	ExpectGe(len(entries), 1)
	entry = entries[0]
	ExpectEq(entry.Status, 0)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
	var lookupOp fuseops.LookUpInodeOp
	loadArgs(entry, &lookupOp)
	ExpectEq(fileName, lookupOp.Name)
	ExpectEq(rootInode, lookupOp.Parent)
	ExpectEq(fileInode, lookupOp.Entry.Child)
	ExpectEq(1, lookupOp.Entry.Attributes.Nlink)
	ExpectEq(len(fileContents), lookupOp.Entry.Attributes.Size)
	ExpectEq(fileMode, lookupOp.Entry.Attributes.Mode)
	ExpectEq("yes", entry.Extra["lookup"])

	// 3. GetInodeAttributesOp
	entries, ok = ops["GetInodeAttributesOp"]
	AssertTrue(ok)
	ExpectGe(len(entries), 2)
	entry = entries[1] // first entry is the root dir
	ExpectEq(0, entry.Status)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
	var getattrOp fuseops.GetInodeAttributesOp
	loadArgs(entry, &getattrOp)
	ExpectEq(fileInode, getattrOp.Inode)
	ExpectEq(1, getattrOp.Attributes.Nlink)
	ExpectEq(len(fileContents), getattrOp.Attributes.Size)
	ExpectEq(fileMode, getattrOp.Attributes.Mode)

	// 4. OpenFileOp
	entries, ok = ops["OpenFileOp"]
	AssertTrue(ok)
	AssertEq(1, len(entries))
	entry = entries[0]
	ExpectEq(0, entry.Status)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
	var openOp fuseops.OpenFileOp
	loadArgs(entry, &openOp)
	ExpectEq(fileInode, openOp.Inode)
	ExpectEq(fileHandle, openOp.Handle)

	// 5. ReadFileOp
	entries, ok = ops["ReadFileOp"]
	AssertTrue(ok)
	AssertEq(1, len(entries))
	entry = entries[0]
	ExpectEq(0, entry.Status)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
	var readOp fuseops.ReadFileOp
	loadArgs(entries[0], &readOp)
	ExpectEq(fileInode, readOp.Inode)
	ExpectEq(fileHandle, readOp.Handle)
	ExpectEq(0, readOp.Offset)
	ExpectGt(readOp.Size, 0)
	ExpectEq(len(fileContents), readOp.BytesRead)

	// 6. FlushFileOp
	entries, ok = ops["FlushFileOp"]
	AssertTrue(ok)
	AssertEq(1, len(entries))
	entry = entries[0]
	ExpectEq(0, entry.Status)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
	var flushOp fuseops.FlushFileOp
	loadArgs(entry, &flushOp)
	ExpectEq(fileInode, flushOp.Inode)
	AssertEq(fileHandle, flushOp.Handle)

	// 7. ReleaseFileHandleOp
	entries, ok = ops["ReleaseFileHandleOp"]
	AssertTrue(ok)
	AssertEq(1, len(entries))
	entry = entries[0]
	ExpectEq(entry.Status, syscall.ENOSYS)
	AssertNe(nil, entry.Context)
	AssertGt(entry.Context.FuseID, 0)
}
