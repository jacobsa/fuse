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
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/googlecloudplatform/gcsfuse/timeutil"
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

// A struct that implements common behavior needed by tests in the samples/
// directory. Use it as an embedded field in your test fixture, calling its
// SetUp method from your SetUp method after setting the FileSystem field.
type SampleTest struct {
	// The file system under test and the configuration with which it should be
	// mounted. These must be set by the user of this type before calling SetUp;
	// all the other fields below are set by SetUp itself.
	FileSystem  fuse.FileSystem
	MountConfig fuse.MountConfig

	// A context object that can be used for long-running operations.
	Ctx context.Context

	// A clock with a fixed initial time. The test's set up method may use this
	// to wire the file system with a clock, if desired.
	Clock timeutil.SimulatedClock

	// The directory at which the file system is mounted.
	Dir string

	mfs *fuse.MountedFileSystem
}

// Mount the supplied file system and initialize the other exported fields of
// the struct. Panics on error.
//
// REQUIRES: t.FileSystem has been set.
func (t *SampleTest) SetUp(ti *ogletest.TestInfo) {
	err := t.initialize(t.FileSystem, &t.MountConfig)
	if err != nil {
		panic(err)
	}
}

// Like Initialize, but doens't panic.
func (t *SampleTest) initialize(
	fs fuse.FileSystem,
	config *fuse.MountConfig) (err error) {
	// Initialize the context.
	t.Ctx = context.Background()

	// Initialize the clock.
	t.Clock.SetTime(time.Date(2012, 8, 15, 22, 56, 0, 0, time.Local))

	// Set up a temporary directory.
	t.Dir, err = ioutil.TempDir("", "sample_test")
	if err != nil {
		err = fmt.Errorf("TempDir: %v", err)
		return
	}

	// Mount the file system.
	t.mfs, err = fuse.Mount(t.Dir, fs, config)
	if err != nil {
		err = fmt.Errorf("Mount: %v", err)
		return
	}

	// Wait for it to be read.
	err = t.mfs.WaitForReady(t.Ctx)
	if err != nil {
		err = fmt.Errorf("WaitForReady: %v", err)
		return
	}

	return
}

// Unmount the file system and clean up. Panics on error.
func (t *SampleTest) TearDown() {
	err := t.destroy()
	if err != nil {
		panic(err)
	}
}

// Like TearDown, but doesn't panic.
func (t *SampleTest) destroy() (err error) {
	// Was the file system mounted?
	if t.mfs == nil {
		return
	}

	// Unmount the file system. Try again on "resource busy" errors.
	delay := 10 * time.Millisecond
	for {
		err = t.mfs.Unmount()
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "resource busy") {
			log.Println("Resource busy error while unmounting; trying again")
			time.Sleep(delay)
			delay = time.Duration(1.3 * float64(delay))
			continue
		}

		err = fmt.Errorf("MountedFileSystem.Unmount: %v", err)
		return
	}

	if err = t.mfs.Join(t.Ctx); err != nil {
		err = fmt.Errorf("MountedFileSystem.Join: %v", err)
		return
	}

	return
}
