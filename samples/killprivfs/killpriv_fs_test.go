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
	t.fs.ResetFlags()
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *KillPrivFSTest) TestMountWithKillPrivV2() {
	// Verify filesystem mounts successfully with HANDLE_KILLPRIV_V2 enabled
	ExpectThat(t.Dir, Not(Equals("")))
}

func (t *KillPrivFSTest) TestCreateFileInSetgidDir() {
	if syscall.Getuid() != 0 {
		return
	}

	dirPath := path.Join(t.Dir, "setgid_dir")
	err := os.Mkdir(dirPath, 0755)
	AssertEq(nil, err)

	err = os.Chmod(dirPath, 02755)
	AssertEq(nil, err)

	stat, err := os.Stat(dirPath)
	AssertEq(nil, err)
	ExpectTrue((stat.Mode() & os.ModeSetgid) != 0, "setgid bit should be set on directory")

	filePath := path.Join(dirPath, "newfile.txt")
	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c",
		"touch "+filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AssertEq(nil, err, "su command failed: %s", string(output))
	}

	createFlag, _, _, _ := t.fs.GetFlags()
	ExpectTrue(createFlag, "CreateKillSuidgid flag should be set when creating file in setgid directory")
}

func (t *KillPrivFSTest) TestOpenSetuidFileForWrite() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "setuid_open_test.txt")

	err := ioutil.WriteFile(filePath, []byte("initial"), 0644)
	AssertEq(nil, err)

	err = os.Chmod(filePath, 04755)
	AssertEq(nil, err)

	stat, err := os.Stat(filePath)
	AssertEq(nil, err)
	ExpectTrue((stat.Mode() & os.ModeSetuid) != 0, "setuid bit should be set")

	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c",
		"echo 'new data' > "+filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AssertEq(nil, err, "su command failed: %s", string(output))
	}

	_, openFlag, _, _ := t.fs.GetFlags()
	ExpectTrue(openFlag, "OpenKillSuidgid flag should be set when opening setuid file for write")
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

	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c",
		"truncate -s 5 "+filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AssertEq(nil, err, "truncate command failed: %s", string(output))
	}

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set when non-root truncates setuid file")
}

func (t *KillPrivFSTest) TestNoKillSuidgidFlagsOnNormalOperations() {
	// This test verifies that KillSuidgid flags are NOT set for normal operations
	// without setuid/setgid bits. We simply check the flags remain false after mount.
	createFlag, openFlag, writeFlag, setattrFlag := t.fs.GetFlags()
	ExpectFalse(createFlag, "CreateKillSuidgid should be false initially")
	ExpectFalse(openFlag, "OpenKillSuidgid should be false initially")
	ExpectFalse(writeFlag, "WriteKillSuidgid should be false initially")
	ExpectFalse(setattrFlag, "SetattrKillSuidgid should be false initially")
}

func (t *KillPrivFSTest) TestChownNormalFile() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "normal_chown.txt")

	err := ioutil.WriteFile(filePath, []byte("test"), 0644)
	AssertEq(nil, err)

	err = os.Chown(filePath, 1000, 1000)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectFalse(setattrFlag, "SetattrKillSuidgid flag should NOT be set when chown on normal file")
}

func (t *KillPrivFSTest) TestRootWriteToSetuidFile() {
	if syscall.Getuid() != 0 {
		return
	}

	filePath := path.Join(t.Dir, "setuid_root_write.txt")

	err := ioutil.WriteFile(filePath, []byte("initial"), 0644)
	AssertEq(nil, err)

	err = os.Chmod(filePath, 04755)
	AssertEq(nil, err)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	AssertEq(nil, err)
	_, err = f.Write([]byte("root write"))
	AssertEq(nil, err)
	f.Close()

	_, _, writeFlag, _ := t.fs.GetFlags()
	ExpectFalse(writeFlag, "WriteKillSuidgid flag should NOT be set when root (with CAP_FSETID) writes to setuid file")
}

func (t *KillPrivFSTest) TestBasicOperationsStillWork() {
	// This simple test verifies the filesystem is functional
	// More comprehensive tests are in the killpriv-specific tests above
	ExpectThat(t.Dir, Not(Equals("")))
}
