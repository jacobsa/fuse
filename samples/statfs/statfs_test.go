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
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse/fuseops"
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

// Ask `df` for statistics about the file system's capacity and free space,
// useful for checking that our reading of statfs(2) output matches the
// system's. The output is not guaranteed to have resolution greater than 2^10
// (1 KiB).
func df(dir string) (capacity, used, available uint64, err error) {
	// Sample output:
	//
	//     Filesystem  1024-blocks Used Available Capacity iused ifree %iused  Mounted on
	//     fake@bucket          32   16        16    50%       0     0  100%   /Users/jacobsa/tmp/mp
	//
	re := regexp.MustCompile(`^\S+\s+(\d+)\s+(\d+)\s+(\d+)\s+\d+%\s+\d+\s+\d+\s+\d+%.*$`)

	// Call df with a block size of 1024 and capture its output.
	cmd := exec.Command("df", dir)
	cmd.Env = []string{"BLOCKSIZE=1024"}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	// Scrape it.
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		// Is this the line we're interested in?
		if !bytes.Contains(line, []byte(dir)) {
			continue
		}

		submatches := re.FindSubmatch(line)
		if submatches == nil {
			err = fmt.Errorf("Unable to parse line: %q", line)
			return
		}

		capacity, err = strconv.ParseUint(string(submatches[1]), 10, 64)
		if err != nil {
			return
		}

		used, err = strconv.ParseUint(string(submatches[2]), 10, 64)
		if err != nil {
			return
		}

		available, err = strconv.ParseUint(string(submatches[3]), 10, 64)
		if err != nil {
			return
		}

		// Scale appropriately based on the BLOCKSIZE set above.
		capacity *= 1024
		used *= 1024
		available *= 1024

		return
	}

	err = fmt.Errorf("Unable to parse df output:\n%s", output)
	return
}

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type StatFSTest struct {
	samples.SampleTest
	fs statfs.FS

	// t.Dir, with symlinks resolved and redundant path components removed.
	canonicalDir string
}

var _ SetUpInterface = &StatFSTest{}
var _ TearDownInterface = &StatFSTest{}

func init() { RegisterTestSuite(&StatFSTest{}) }

func (t *StatFSTest) SetUp(ti *TestInfo) {
	var err error

	// Writeback caching can ruin our measurement of the write sizes the kernel
	// decides to give us, since it causes write acking to race against writes
	// being issued from the client.
	t.MountConfig.DisableWritebackCaching = true

	// Create the file system.
	t.fs = statfs.New()
	t.Server = fuseutil.NewFileSystemServer(t.fs)

	// Mount it.
	t.SampleTest.SetUp(ti)

	// Canonicalize the mount point.
	t.canonicalDir, err = filepath.EvalSymlinks(t.Dir)
	AssertEq(nil, err)
	t.canonicalDir = path.Clean(t.canonicalDir)
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *StatFSTest) Syscall_ZeroValues() {
	var err error
	var stat syscall.Statfs_t

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
	ExpectEq(t.canonicalDir, convertName(stat.Mntonname[:]))
	ExpectThat(
		convertName(stat.Mntfromname[:]),
		MatchesRegexp(`mount_osxfusefs@osxfuse\d+`))
}

func (t *StatFSTest) Syscall_NonZeroValues() {
	var err error
	var stat syscall.Statfs_t

	// Set up the canned response.
	canned := fuseops.StatFSOp{
		BlockSize: 1 << 15,

		Blocks:          1<<51 + 3,
		BlocksFree:      1<<43 + 5,
		BlocksAvailable: 1<<41 + 7,

		Inodes:     1<<59 + 11,
		InodesFree: 1<<58 + 13,
	}

	t.fs.SetStatFSResponse(canned)

	// Stat.
	err = syscall.Statfs(t.Dir, &stat)
	AssertEq(nil, err)

	ExpectEq(canned.BlockSize, stat.Bsize)
	ExpectEq(canned.BlockSize, stat.Iosize)
	ExpectEq(canned.Blocks, stat.Blocks)
	ExpectEq(canned.BlocksFree, stat.Bfree)
	ExpectEq(canned.BlocksAvailable, stat.Bavail)
	ExpectEq(canned.Inodes, stat.Files)
	ExpectEq(canned.InodesFree, stat.Ffree)
	ExpectEq("osxfusefs", convertName(stat.Fstypename[:]))
	ExpectEq(t.canonicalDir, convertName(stat.Mntonname[:]))
	ExpectThat(
		convertName(stat.Mntfromname[:]),
		MatchesRegexp(`mount_osxfusefs@osxfuse\d+`))
}

func (t *StatFSTest) CapacityAndFreeSpace() {
	canned := fuseops.StatFSOp{
		Blocks:          1024,
		BlocksFree:      896,
		BlocksAvailable: 768,
	}

	// Check that df agrees with us about a range of block sizes.
	for log2BlockSize := uint(9); log2BlockSize <= 17; log2BlockSize++ {
		bs := uint64(1) << log2BlockSize
		desc := fmt.Sprintf("block size: %d (2^%d)", bs, log2BlockSize)

		// Set up the canned response.
		canned.BlockSize = uint32(bs)
		t.fs.SetStatFSResponse(canned)

		// Call df.
		capacity, used, available, err := df(t.canonicalDir)
		AssertEq(nil, err)

		ExpectEq(bs*canned.Blocks, capacity, "%s", desc)
		ExpectEq(bs*(canned.Blocks-canned.BlocksFree), used, "%s", desc)
		ExpectEq(bs*canned.BlocksAvailable, available, "%s", desc)
	}
}

func (t *StatFSTest) WriteSize() {
	var err error

	// Set up a smallish block size.
	canned := fuseops.StatFSOp{
		BlockSize:       8192,
		Blocks:          1234,
		BlocksFree:      1234,
		BlocksAvailable: 1234,
	}

	t.fs.SetStatFSResponse(canned)

	// Cause a large amount of date to be written.
	err = ioutil.WriteFile(
		path.Join(t.Dir, "foo"),
		bytes.Repeat([]byte{'x'}, 1<<22),
		0400)

	AssertEq(nil, err)

	// Despite the small block size, the OS shouldn't have given us pitifully
	// small chunks of data.
	ExpectEq(1<<20, t.fs.MostRecentWriteSize())
}

func (t *StatFSTest) UnsupportedBlockSizes() {
	var err error

	// Test a bunch of block sizes that the OS doesn't support faithfully,
	// checking what it transforms them too.
	testCases := []struct {
		fsBlockSize    uint32
		expectedBsize  uint32
		expectedIosize uint32
	}{
		0:  {0, 4096, 65536},
		1:  {1, 512, 512},
		2:  {3, 512, 512},
		3:  {511, 512, 512},
		4:  {513, 1024, 1024},
		5:  {1023, 1024, 1024},
		6:  {4095, 4096, 4096},
		7:  {1<<17 - 1, 1 << 17, 131072},
		8:  {1<<17 + 1, 1 << 17, 1 << 18},
		9:  {1<<18 + 1, 1 << 17, 1 << 19},
		10: {1<<19 + 1, 1 << 17, 1 << 20},
		11: {1<<20 + 1, 1 << 17, 1 << 20},
		12: {1 << 21, 1 << 17, 1 << 20},
		13: {1 << 30, 1 << 17, 1 << 20},
	}

	for i, tc := range testCases {
		desc := fmt.Sprintf("Case %d: block size %d", i, tc.fsBlockSize)

		// Set up.
		canned := fuseops.StatFSOp{
			BlockSize: tc.fsBlockSize,
			Blocks:    10,
		}

		t.fs.SetStatFSResponse(canned)

		// Check.
		var stat syscall.Statfs_t
		err = syscall.Statfs(t.Dir, &stat)
		AssertEq(nil, err)

		ExpectEq(tc.expectedBsize, stat.Bsize, "%s", desc)
		ExpectEq(tc.expectedIosize, stat.Iosize, "%s", desc)
	}
}
