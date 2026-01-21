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

// Package killprivfs_test verifies FUSE HANDLE_KILLPRIV_V2 support.
//
// These tests verify that the kernel sets KillSuidgid flags on FUSE operations
// when appropriate. The tests create real setuid files and use privilege
// dropping to trigger the kernel behavior.
//
// Note: Actual behavior depends on kernel version (feature introduced in
// Linux 5.12), whether the filesystem advertises FUSE_HANDLE_KILLPRIV_V2,
// and user capabilities (CAP_FSETID).
package killprivfs_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/killprivfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
)

func TestKillPrivFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type KillPrivFSTest struct {
	samples.SampleTest
	fs *killprivfs.KillPrivFS
}

func init() { RegisterTestSuite(&KillPrivFSTest{}) }

func (t *KillPrivFSTest) SetUp(ti *TestInfo) {
	t.fs = killprivfs.NewKillPrivFS()
	t.Server = fuseutil.NewFileSystemServer(t.fs)
	t.MountConfig.EnableHandleKillprivV2 = true
	t.SampleTest.SetUp(ti)
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *KillPrivFSTest) TestMountWithKillPrivV2() {
	// Verify filesystem mounts successfully with HANDLE_KILLPRIV_V2 enabled
	ExpectThat(t.Dir, Not(Equals("")))
}

func (t *KillPrivFSTest) TestWriteToSetuidFile() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "setuid_test.txt")

	err := ioutil.WriteFile(filePath, []byte("initial"), 0644)
	AssertEq(nil, err)

	err = os.Chmod(filePath, 04755)
	AssertEq(nil, err)

	stat, err := os.Stat(filePath)
	AssertEq(nil, err)
	ExpectTrue((stat.Mode() & os.ModeSetuid) != 0, "setuid bit should be set")

	t.fs.ResetFlags()

	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c",
		"echo 'test data' >> "+filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AssertEq(nil, err, "su command failed: %s", string(output))
	}

	_, _, writeFlag, _ := t.fs.GetFlags()
	ExpectTrue(writeFlag, "WriteKillSuidgid flag should be set when non-root writes to setuid file")
}

func (t *KillPrivFSTest) TestSetuidBitWithChown() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "chown_test.txt")

	err := ioutil.WriteFile(filePath, []byte("test"), 0644)
	AssertEq(nil, err)

	err = os.Chmod(filePath, 04755)
	AssertEq(nil, err)

	t.fs.ResetFlags()

	err = os.Chown(filePath, 1000, 1000)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set when changing ownership of setuid file")
}

func (t *KillPrivFSTest) TestTruncateSetuidFile() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "truncate_test.txt")

	err := ioutil.WriteFile(filePath, []byte("test data here"), 0644)
	AssertEq(nil, err)

	err = os.Chmod(filePath, 04755)
	AssertEq(nil, err)

	t.fs.ResetFlags()

	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c",
		"truncate -s 5 "+filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AssertEq(nil, err, "truncate command failed: %s", string(output))
	}

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set when non-root truncates setuid file")
}

func (t *KillPrivFSTest) TestBasicOperationsStillWork() {
	// Verify that normal operations work correctly even with KILLPRIV_V2 enabled
	filePath := path.Join(t.Dir, "normal_test.txt")

	// Create a normal file
	err := ioutil.WriteFile(filePath, []byte("test"), 0644)
	AssertEq(nil, err)

	// Read it back
	content, err := ioutil.ReadFile(filePath)
	AssertEq(nil, err)
	ExpectEq("test", string(content))

	// Truncate it
	err = os.Truncate(filePath, 2)
	AssertEq(nil, err)

	// Verify truncate worked
	stat, err := os.Stat(filePath)
	AssertEq(nil, err)
	ExpectEq(2, stat.Size())
}
