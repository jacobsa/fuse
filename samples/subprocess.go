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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"sync"

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

	mountCmd    *exec.Cmd
	mountStdout bytes.Buffer
	mountStderr bytes.Buffer
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

	// Set up a command.
	args := []string{
		"--type",
		t.MountType,
	}

	args = append(args, t.MountFlags...)

	t.mountCmd = exec.Command(toolPath, args...)
	t.mountCmd.Stdout = &t.mountStdout
	t.mountCmd.Stderr = &t.mountStderr

	// Start it.
	if err = t.mountCmd.Start(); err != nil {
		err = fmt.Errorf("mountCmd.Start: %v", err)
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

	// If we didn't try to mount the file system, there's nothing further to do.
	if t.mountCmd == nil {
		return
	}

	// In the background, initiate an unmount.
	unmountErrChan := make(chan error)
	go func() {
		unmountErrChan <- unmount(t.Dir)
	}()

	// Make sure we wait for the unmount, even if we've already returned early in
	// error. Return its error if we haven't seen any other error.
	defer func() {
		unmountErr := <-unmountErrChan
		if unmountErr != nil {
			if err != nil {
				log.Println("unmount:", unmountErr)
				return
			}

			err = fmt.Errorf("unmount: %v", unmountErr)
		}
	}()

	// Wait for the subprocess.
	if err = t.mountCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf(
				"mount_sample exited with %v. Stderr:\n%s",
				exitErr,
				t.mountStderr.String())

			return
		}

		err = fmt.Errorf("mountCmd.Wait: %v", err)
		return
	}

	return
}
