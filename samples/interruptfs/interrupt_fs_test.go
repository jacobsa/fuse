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

package interruptfs_test

import (
	"os"
	"path"
	"testing"

	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/interruptfs"
	. "github.com/jacobsa/ogletest"
)

func TestInterruptFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type InterruptFSTest struct {
	samples.SampleTest
	fs *interruptfs.InterruptFS
}

func init() { RegisterTestSuite(&InterruptFSTest{}) }

var _ SetUpInterface = &InterruptFSTest{}
var _ TearDownInterface = &InterruptFSTest{}

func (t *InterruptFSTest) SetUp(ti *TestInfo) {
	var err error

	// Create the file system.
	t.fs = interruptfs.New()
	AssertEq(nil, err)

	t.Server = fuseutil.NewFileSystemServer(t.fs)

	// Mount it.
	t.SampleTest.SetUp(ti)
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *InterruptFSTest) StatFoo() {
	fi, err := os.Stat(path.Join(t.Dir, "foo"))
	AssertEq(nil, err)

	ExpectEq("foo", fi.Name())
	ExpectEq(0777, fi.Mode())
	ExpectFalse(fi.IsDir())
}

func (t *InterruptFSTest) InterruptedDuringRead() {
	AssertTrue(false, "TODO")
}
