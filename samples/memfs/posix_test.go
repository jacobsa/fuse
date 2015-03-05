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

// Tests for the behavior of os.File objects on plain old posix file systems,
// for use in verifying the intended behavior of memfs.

package memfs_test

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	. "github.com/jacobsa/ogletest"
)

func TestPosix(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type PosixTest struct {
	// A temporary directory.
	dir string

	// Files to close when tearing down. Nil entries are skipped.
	toClose []io.Closer
}

var _ SetUpInterface = &PosixTest{}
var _ TearDownInterface = &PosixTest{}

func init() { RegisterTestSuite(&PosixTest{}) }

func (t *PosixTest) SetUp(ti *TestInfo) {
	var err error

	// Create a temporary directory.
	t.dir, err = ioutil.TempDir("", "posix_test")
	if err != nil {
		panic(err)
	}
}

func (t *PosixTest) TearDown() {
	// Close any files we opened.
	for _, c := range t.toClose {
		if c == nil {
			continue
		}

		err := c.Close()
		if err != nil {
			panic(err)
		}
	}

	// Remove the temporary directory.
	err := os.RemoveAll(t.dir)
	if err != nil {
		panic(err)
	}
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *PosixTest) WriteOverlapsEndOfFile() {
	var err error
	var n int

	// Create a file.
	f, err := os.Create(path.Join(t.dir, "foo"))
	t.toClose = append(t.toClose, f)
	AssertEq(nil, err)

	// Make it 4 bytes long.
	err = f.Truncate(4)
	AssertEq(nil, err)

	// Write the range [2, 6).
	n, err = f.WriteAt([]byte("taco"), 2)
	AssertEq(nil, err)
	AssertEq(4, n)

	// Read the full contents of the file.
	contents, err := ioutil.ReadAll(f)
	AssertEq(nil, err)
	ExpectEq("\x00\x00taco", string(contents))
}

func (t *PosixTest) WriteStartsAtEndOfFile() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) WriteStartsPastEndOfFile() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) WriteAtEffectOnOffset_NotAppendMode() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) WriteAtEffectOnOffset_AppendMode() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) ReadOverlapsEndOfFile() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) ReadStartsAtEndOfFile() {
	AssertTrue(false, "TODO")
}

func (t *PosixTest) ReadStartsPastEndOfFile() {
	AssertTrue(false, "TODO")
}
