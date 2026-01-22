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

// skipIfNotRoot skips the test if not running as root.
func skipIfNotRoot(testName string) bool {
	if syscall.Getuid() != 0 {
		fmt.Printf("Skipping %s: requires root\n", testName)
		return true
	}
	return false
}

// skipIfKernelTooOld skips the test if kernel doesn't support KILLPRIV_V2.
func skipIfKernelTooOld(testName string) bool {
	if !kernelSupportsKillprivV2() {
		fmt.Printf("Skipping %s: requires Linux kernel >= 5.12 for HANDLE_KILLPRIV_V2 support\n", testName)
		return true
	}
	return false
}

// runAsNobody executes a shell command as the nobody user.
func runAsNobody(command string) error {
	cmd := exec.Command("su", "-s", "/bin/sh", "nobody", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s", string(output))
	}
	return nil
}

func (t *KillPrivFSTest) SetUp(ti *TestInfo) {
	t.fs = killprivfs.NewKillPrivFS()
	t.Server = fuseutil.NewFileSystemServer(t.fs)
	t.MountConfig.EnableHandleKillprivV2 = true

	// IMPORTANT: DisableWritebackCaching must be true for KILLPRIV_V2 to work correctly.
	// With writeback caching enabled, the kernel buffers writes in the page cache and
	// KillSuidgid flags don't reach the filesystem until much later (if at all).
	t.MountConfig.DisableWritebackCaching = true

	// Allow other users to access the filesystem (required for privilege dropping tests)
	if t.MountConfig.Options == nil {
		t.MountConfig.Options = make(map[string]string)
	}
	t.MountConfig.Options["allow_other"] = ""

	t.SampleTest.SetUp(ti)

	err := os.Chmod(t.Dir, 0755)
	AssertEq(nil, err, "Failed to chmod mount point")

	t.fs.ResetFlags()
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *KillPrivFSTest) TestMountWithKillPrivV2() {
	ExpectThat(t.Dir, Not(Equals("")))
}

func (t *KillPrivFSTest) TestCreateFileInSetgidDir() {
	if skipIfNotRoot("TestCreateFileInSetgidDir") || skipIfKernelTooOld("TestCreateFileInSetgidDir") {
		return
	}

	// Shell > redirection uses O_CREAT|O_TRUNC (without O_EXCL), which triggers
	// the kernel to set KillSuidgid when creating in a setgid directory
	t.fs.AddTestDir("setgid_dir", 02777)
	filePath := path.Join(t.Dir, "setgid_dir", "newfile.txt")

	err := runAsNobody("echo data > " + filePath)
	AssertEq(nil, err)

	createFlag, _, _, _ := t.fs.GetFlags()
	ExpectTrue(createFlag, "CreateKillSuidgid flag should be set when creating file in setgid directory with O_TRUNC")
}

func (t *KillPrivFSTest) TestOpenWithTruncateSetuidFile() {
	if skipIfNotRoot("TestOpenWithTruncateSetuidFile") || skipIfKernelTooOld("TestOpenWithTruncateSetuidFile") {
		return
	}

	// When opening with O_TRUNC, the kernel splits it into OpenFile + SetInodeAttributes(size=0).
	// The KillSuidgid flag is set on SetInodeAttributes, not OpenFile.
	t.fs.AddTestFile("setuid_open_test.txt", 04666)
	filePath := path.Join(t.Dir, "setuid_open_test.txt")

	err := runAsNobody("echo data > " + filePath)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set when truncating setuid file via O_TRUNC")
}

func (t *KillPrivFSTest) TestWriteToSetuidFile() {
	if skipIfNotRoot("TestWriteToSetuidFile") || skipIfKernelTooOld("TestWriteToSetuidFile") {
		return
	}

	t.fs.AddTestFile("setuid_test.txt", 04666)
	filePath := path.Join(t.Dir, "setuid_test.txt")

	err := runAsNobody("echo 'test data' >> " + filePath)
	AssertEq(nil, err)

	_, _, writeFlag, _ := t.fs.GetFlags()
	ExpectTrue(writeFlag, "WriteKillSuidgid flag should be set when non-root writes to setuid file")
}

func (t *KillPrivFSTest) TestSetuidBitWithChown() {
	if skipIfNotRoot("TestSetuidBitWithChown") || skipIfKernelTooOld("TestSetuidBitWithChown") {
		return
	}

	// The kernel always sets KillSuidgid=true for setattr operations.
	// The filesystem must check OpContext.Uid and file mode to decide if bits should clear.
	t.fs.AddTestFile("chown_test.txt", 04755)
	filePath := path.Join(t.Dir, "chown_test.txt")

	err := os.Chown(filePath, 1000, 1000)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set for setattr (chown) operation")
}

func (t *KillPrivFSTest) TestTruncateSetuidFile() {
	if skipIfNotRoot("TestTruncateSetuidFile") || skipIfKernelTooOld("TestTruncateSetuidFile") {
		return
	}

	t.fs.AddTestFile("truncate_test.txt", 04666)
	filePath := path.Join(t.Dir, "truncate_test.txt")

	err := runAsNobody("truncate -s 5 " + filePath)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set when non-root truncates setuid file")
}

func (t *KillPrivFSTest) TestNoKillSuidgidFlagsOnNormalOperations() {
	createFlag, openFlag, writeFlag, setattrFlag := t.fs.GetFlags()
	ExpectFalse(createFlag, "CreateKillSuidgid should be false initially")
	ExpectFalse(openFlag, "OpenKillSuidgid should be false initially")
	ExpectFalse(writeFlag, "WriteKillSuidgid should be false initially")
	ExpectFalse(setattrFlag, "SetattrKillSuidgid should be false initially")
}

func (t *KillPrivFSTest) TestChownNormalFile() {
	if skipIfNotRoot("TestChownNormalFile") || skipIfKernelTooOld("TestChownNormalFile") {
		return
	}

	// The kernel sets KillSuidgid=true even on normal files (without privilege bits).
	// The filesystem must check the file mode to decide if any action is needed.
	filePath := path.Join(t.Dir, "normal_chown.txt")

	err := ioutil.WriteFile(filePath, []byte("test"), 0644)
	AssertEq(nil, err)

	err = os.Chown(filePath, 1000, 1000)
	AssertEq(nil, err)

	_, _, _, setattrFlag := t.fs.GetFlags()
	ExpectTrue(setattrFlag, "SetattrKillSuidgid flag should be set for setattr (chown) operation")
}

func (t *KillPrivFSTest) TestRootWriteToSetuidFile() {
	if skipIfNotRoot("TestRootWriteToSetuidFile") {
		return
	}

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
