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

package statfs_test

import (
	"testing"

	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/statfs"
	. "github.com/jacobsa/ogletest"
)

func TestStatFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type StatFSTest struct {
	samples.SampleTest
	fs statfs.FS
}

var _ SetUpInterface = &StatFSTest{}
var _ TearDownInterface = &StatFSTest{}

func init() { RegisterTestSuite(&StatFSTest{}) }

func (t *StatFSTest) SetUp(ti *TestInfo) {
	// Writeback caching can ruin our measurement of the write sizes the kernel
	// decides to give us, since it causes write acking to race against writes
	// being issued from the client.
	t.MountConfig.DisableWritebackCaching = true

	// Create the file system.
	t.fs = statfs.New()
	t.Server = fuseutil.NewFileSystemServer(t.fs)

	// Mount it.
	t.SampleTest.SetUp(ti)
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *StatFSTest) Syscall_ZeroValues() {
	AssertTrue(false, "TODO")
}

func (t *StatFSTest) Syscall_NonZeroValues() {
	AssertTrue(false, "TODO")
}

func (t *StatFSTest) CapacityAndFreeSpace() {
	AssertTrue(false, "TODO: Use df")
}

func (t *StatFSTest) WriteSize() {
	AssertTrue(false, "TODO")
}

func (t *StatFSTest) UnsupportedBlockSizes() {
	AssertTrue(false, "TODO")
}
