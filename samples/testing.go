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
	"github.com/googlecloudplatform/gcsfuse/timeutil"
	"github.com/jacobsa/fuse"
)

// A struct that implements common behavior needed by tests in the samples/
// directory. Use it as an anonymous member of your test fixture, calling its
// Initialize method from your SetUp method and its Destroy method from your
// TearDown method.
type SampleTest struct {
	// A clock with a fixed initial time. The test's set up method may use this
	// to wire the file system with a clock, if desired.
	Clock timeutil.SimulatedClock

	// The directory at which the file system is mounted.
	Dir string
}

// Mount the supplied file system and initialize the exported fields of the
// struct. Panics on error.
func (st *SampleTest) Initialize(fs fuse.FileSystem)

// Unmount the file system and clean up. Panics on error.
func (st *SampleTest) Destroy()
