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
	"path"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/statfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
)

func TestStatFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

func convertName(in []int8) (s string) {
	var tmp []byte
	for _, v := range in {
		if v == 0 {
			break
		}

		tmp = append(tmp, byte(v))
	}

	s = string(tmp)
	return
}

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
	var err error
	var stat syscall.Statfs_t

	// Compute the canonicalized directory, for use below.
	canonicalDir, err := filepath.EvalSymlinks(t.Dir)
	AssertEq(nil, err)
	canonicalDir = path.Clean(canonicalDir)

	// Call without configuring a canned response, meaning the OS will see the
	// zero value for each field. The assertions below act as documentation for
	// the OS's behavior in this case.
	err = syscall.Statfs(t.Dir, &stat)
	AssertEq(nil, err)

	ExpectEq(4096, stat.Bsize)
	ExpectEq(65536, stat.Iosize)
	ExpectEq(0, stat.Blocks)
	ExpectEq(0, stat.Bfree)
	ExpectEq(0, stat.Bavail)
	ExpectEq(0, stat.Files)
	ExpectEq(0, stat.Ffree)
	ExpectEq("osxfusefs", convertName(stat.Fstypename[:]))
	ExpectEq(canonicalDir, convertName(stat.Mntonname[:]))
	ExpectThat(
		convertName(stat.Mntfromname[:]),
		MatchesRegexp(`mount_osxfusefs@osxfuse\d+`))
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
