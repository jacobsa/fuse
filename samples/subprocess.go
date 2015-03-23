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

package samples

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

// A struct that implements common behavior needed by tests in the samples/
// directory where the file system is mounted by a subprocess. Use it as an
// embedded field in your test fixture, calling its SetUp method from your
// SetUp method after setting the MountType and MountFlags fields.
type SubprocessTest struct {
	// The type of the file system to mount. Must be recognized by mount_sample.
	MountType string

	// Additional flags to be passed to the mount_sample tool.
	MountFlags []string

	// A context object that can be used for long-running operations.
	Ctx context.Context

	// The directory at which the file system is mounted.
	Dir string

	// Anothing non-nil in this slice will be closed by TearDown. The test will
	// fail if closing fails.
	ToClose []io.Closer

	mountCmd *exec.Cmd
}

// Mount the file system and initialize the other exported fields of the
// struct. Panics on error.
//
// REQUIRES: t.FileSystem has been set.
func (t *SubprocessTest) SetUp(ti *ogletest.TestInfo) {
	err := t.initialize()
	if err != nil {
		panic(err)
	}
}

// Set by buildMountSample.
var mountSamplePath string
var mountSampleErr error
var mountSampleOnce sync.Once

// Build the mount_sample tool if it has not yet been built for this process.
// Return a path to the binary.
func buildMountSample() (toolPath string, err error) {
	// Build if we haven't yet.
	mountSampleOnce.Do(func() {
		// Create a temporary directory.
		tempDir, err := ioutil.TempDir("", "")
		if err != nil {
			mountSampleErr = fmt.Errorf("TempDir: %v", err)
			return
		}

		mountSamplePath = path.Join(tempDir, "mount_sample")

		// Build the command.
		cmd := exec.Command(
			"go",
			"build",
			"-o",
			mountSamplePath,
			"github.com/jacobsa/fuse/samples/mount_sample")

		output, err := cmd.CombinedOutput()
		if err != nil {
			mountSampleErr = fmt.Errorf(
				"mount_sample exited with %v, output:\n%s",
				err,
				string(output))

			return
		}
	})

	if mountSampleErr != nil {
		err = mountSampleErr
		return
	}

	toolPath = mountSamplePath
	return
}

// Invoke mount_sample, returning a running command.
func invokeMountSample(path string, args []string) (cmd *exec.Cmd, err error) {
	cmd = exec.Command(path, args...)
	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("Start: %v", err)
		return
	}

	return
}

// Like SetUp, but doens't panic.
func (t *SubprocessTest) initialize() (err error) {
	// Initialize the context.
	t.Ctx = context.Background()

	// Set up a temporary directory.
	t.Dir, err = ioutil.TempDir("", "sample_test")
	if err != nil {
		err = fmt.Errorf("TempDir: %v", err)
		return
	}

	// Build the mount_sample tool.
	toolPath, err := buildMountSample()
	if err != nil {
		err = fmt.Errorf("buildMountSample: %v", err)
		return
	}

	// Invoke it.
	args := []string{"--type", t.MountType}
	args = append(args, t.MountFlags...)

	t.mountCmd, err = invokeMountSample(toolPath, args)
	if err != nil {
		err = fmt.Errorf("invokeMountSample: %v", err)
		return
	}

	// TODO(jacobsa): Probably need some sort of signalling (on stderr? write to
	// a flag-controlled file?) when WaitForReady has returned.

	return
}

// Unmount the file system and clean up. Panics on error.
func (t *SubprocessTest) TearDown() {
	err := t.destroy()
	if err != nil {
		panic(err)
	}
}

// Like TearDown, but doesn't panic.
func (t *SubprocessTest) destroy() (err error) {
	// Close what is necessary.
	for _, c := range t.ToClose {
		if c == nil {
			continue
		}

		ogletest.ExpectEq(nil, c.Close())
	}

	// Was the file system mounted?
	if t.mountCmd == nil {
		return
	}

	// Unmount the file system. Try again on "resource busy" errors.
	delay := 10 * time.Millisecond
	for {
		err = bazilfuse.Unmount(t.Dir)
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "resource busy") {
			log.Println("Resource busy error while unmounting; trying again")
			time.Sleep(delay)
			delay = time.Duration(1.3 * float64(delay))
			continue
		}

		err = fmt.Errorf("Unmount: %v", err)
		return
	}

	// Wait for the subprocess.
	if err = t.mountCmd.Wait(); err != nil {
		err = fmt.Errorf("Cmd.Wait: %v", err)
		return
	}

	return
}
