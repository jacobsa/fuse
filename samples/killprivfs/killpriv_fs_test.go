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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/killprivfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
)

func TestKillPrivFS(t *testing.T) { RunTests(t) }

// kernelSupportsKillprivV2 checks if the Linux kernel is >= 5.12,
// which is when HANDLE_KILLPRIV_V2 support was added.
func kernelSupportsKillprivV2() bool {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return false
	}

	// Convert release string to Go string (it's a [65]int8)
	releaseBytes := make([]byte, 0, 65)
	for _, b := range uname.Release {
		if b == 0 {
			break
		}
		releaseBytes = append(releaseBytes, byte(b))
	}
	release := string(releaseBytes)

	// Parse version numbers (e.g., "5.12.0-generic" -> major=5, minor=12)
	parts := strings.Split(release, ".")
	if len(parts) < 2 {
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	// HANDLE_KILLPRIV_V2 was added in Linux 5.12
	return major > 5 || (major == 5 && minor >= 12)
}

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
	t.MountConfig.DisableDefaultPermissions = true

	// Allow other users to access the filesystem (required for su nobody tests)
	if t.MountConfig.Options == nil {
		t.MountConfig.Options = make(map[string]string)
	}
	t.MountConfig.Options["allow_other"] = ""

	t.SampleTest.SetUp(ti)

	// Make mount point accessible to all users so tests with `su nobody` work
	err := os.Chmod(t.Dir, 0755)
	AssertEq(nil, err, "Failed to chmod mount point")

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
		fmt.Println("Skipping TestCreateFileInSetgidDir: requires root")
		return
	}
	if !kernelSupportsKillprivV2() {
		fmt.Println("Skipping TestCreateFileInSetgidDir: requires Linux kernel >= 5.12 for HANDLE_KILLPRIV_V2 support")
		return
	}

	// Directly add a directory with setgid bit to the filesystem
	// Use 02777 so nobody user can create files in it
	t.fs.AddTestDir("setgid_dir", 02777)

	dirPath := path.Join(t.Dir, "setgid_dir")
	filePath := path.Join(dirPath, "newfile.txt")

	// As nobody user, create a file in the setgid directory
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
		fmt.Println("Skipping TestOpenSetuidFileForWrite: requires root")
		return
	}
	if !kernelSupportsKillprivV2() {
		fmt.Println("Skipping TestOpenSetuidFileForWrite: requires Linux kernel >= 5.12 for HANDLE_KILLPRIV_V2 support")
		return
	}

	// Directly add a file with setuid bit to the filesystem
	// Use 04666 so nobody user can write to it
	t.fs.AddTestFile("setuid_open_test.txt", 04666)

	filePath := path.Join(t.Dir, "setuid_open_test.txt")

	// As nobody user, open the setuid file for write
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
		fmt.Println("Skipping TestWriteToSetuidFile: requires root")
		return
	}
	if !kernelSupportsKillprivV2() {
		fmt.Println("Skipping TestWriteToSetuidFile: requires Linux kernel >= 5.12 for HANDLE_KILLPRIV_V2 support")
		return
	}

	// Directly add a file with setuid bit to the filesystem
	// Use 04666 so nobody user can write to it
	t.fs.AddTestFile("setuid_test.txt", 04666)

	filePath := path.Join(t.Dir, "setuid_test.txt")

	// As nobody user, write to the setuid file
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
		fmt.Println("Skipping TestSetuidBitWithChown: requires root")
		return
	}

	// Directly add a file with setuid bit to the filesystem
	t.fs.AddTestFile("chown_test.txt", 04755)

	filePath := path.Join(t.Dir, "chown_test.txt")

	err := os.Chown(filePath, 1000, 1000)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	// Root has CAP_FSETID, so the flag should NOT be set when root does chown
	ExpectFalse(setattrFlag, "SetattrKillSuidgid flag should NOT be set when root (with CAP_FSETID) changes ownership")
}

func (t *KillPrivFSTest) TestTruncateSetuidFile() {
	if syscall.Getuid() != 0 {
		fmt.Println("Skipping TestTruncateSetuidFile: requires root")
		return
	}
	if !kernelSupportsKillprivV2() {
		fmt.Println("Skipping TestTruncateSetuidFile: requires Linux kernel >= 5.12 for HANDLE_KILLPRIV_V2 support")
		return
	}

	// Directly add a file with setuid bit to the filesystem
	// Use 04666 so nobody user can write to it
	t.fs.AddTestFile("truncate_test.txt", 04666)

	filePath := path.Join(t.Dir, "truncate_test.txt")

	// As nobody user, truncate the setuid file
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
		fmt.Println("Skipping TestChownNormalFile: requires root")
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
		fmt.Println("Skipping TestRootWriteToSetuidFile: requires root")
		return
	}

	// Directly add a file with setuid bit to the filesystem
	t.fs.AddTestFile("setuid_root_write.txt", 04755)

	filePath := path.Join(t.Dir, "setuid_root_write.txt")

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	AssertEq(nil, err)
	_, err = f.Write([]byte("root write"))
	AssertEq(nil, err)
	f.Close()

	_, _, writeFlag, _ := t.fs.GetFlags()
	ExpectFalse(writeFlag, "WriteKillSuidgid flag should NOT be set when root (with CAP_FSETID) writes to setuid file")
}
