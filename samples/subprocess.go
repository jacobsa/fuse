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
	"io"

	"github.com/jacobsa/fuse"
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

	mfs *fuse.MountedFileSystem
}
