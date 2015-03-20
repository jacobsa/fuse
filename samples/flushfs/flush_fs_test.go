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

package flushfs_test

import (
	"io"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/flushfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
)

func TestFlushFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type FlushFSTest struct {
	samples.SampleTest

	mu sync.Mutex

	// GUARDED_BY(mu)
	flushes  []string
	flushErr error

	// GUARDED_BY(mu)
	fsyncs   []string
	fsyncErr error
}

func init() { RegisterTestSuite(&FlushFSTest{}) }

func (t *FlushFSTest) SetUp(ti *TestInfo) {
	var err error

	// Set up a file system.
	reportTo := func(slice *[]string, err *error) func(string) error {
		return func(s string) error {
			t.mu.Lock()
			defer t.mu.Unlock()

			*slice = append(*slice, s)
			return *err
		}
	}

	t.FileSystem, err = flushfs.NewFileSystem(
		reportTo(&t.flushes, &t.flushErr),
		reportTo(&t.fsyncs, &t.fsyncErr))

	if err != nil {
		panic(err)
	}

	// Mount it.
	t.SampleTest.SetUp(ti)
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

// Return a copy of the current contents of t.flushes.
//
// LOCKS_EXCLUDED(t.mu)
func (t *FlushFSTest) getFlushes() (p []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	p = make([]string, len(t.flushes))
	copy(p, t.flushes)
	return
}

// Return a copy of the current contents of t.fsyncs.
//
// LOCKS_EXCLUDED(t.mu)
func (t *FlushFSTest) getFsyncs() (p []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	p = make([]string, len(t.fsyncs))
	copy(p, t.fsyncs)
	return
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (t *FlushFSTest) CloseReports_ReadWrite() {
	var n int
	var off int64
	var err error
	buf := make([]byte, 1024)

	// Open the file.
	f, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_RDWR, 0)
	AssertEq(nil, err)

	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	// Write some contents to the file.
	n, err = f.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	// Seek and read them back.
	off, err = f.Seek(0, 0)
	AssertEq(nil, err)
	AssertEq(0, off)

	n, err = f.Read(buf)
	AssertThat(err, AnyOf(nil, io.EOF))
	AssertEq("taco", string(buf[:n]))

	// At this point, no flushes or fsyncs should have happened.
	AssertThat(t.getFlushes(), ElementsAre())
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the file.
	err = f.Close()
	f = nil
	AssertEq(nil, err)

	// Now we should have received the flush operation (but still no fsync).
	ExpectThat(t.getFlushes(), ElementsAre("taco"))
	ExpectThat(t.getFsyncs(), ElementsAre())
}

func (t *FlushFSTest) CloseReports_ReadOnly() {
	var err error

	// Open the file.
	f, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_RDONLY, 0)
	AssertEq(nil, err)

	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	// At this point, no flushes or fsyncs should have happened.
	AssertThat(t.getFlushes(), ElementsAre())
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the file.
	err = f.Close()
	f = nil
	AssertEq(nil, err)

	// Now we should have received the flush operation (but still no fsync).
	ExpectThat(t.getFlushes(), ElementsAre(""))
	ExpectThat(t.getFsyncs(), ElementsAre())
}

func (t *FlushFSTest) CloseReports_WriteOnly() {
	var n int
	var err error

	// Open the file.
	f, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_WRONLY, 0)
	AssertEq(nil, err)

	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	// Write some contents to the file.
	n, err = f.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	// At this point, no flushes or fsyncs should have happened.
	AssertThat(t.getFlushes(), ElementsAre())
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the file.
	err = f.Close()
	f = nil
	AssertEq(nil, err)

	// Now we should have received the flush operation (but still no fsync).
	ExpectThat(t.getFlushes(), ElementsAre("taco"))
	ExpectThat(t.getFsyncs(), ElementsAre())
}

func (t *FlushFSTest) CloseReports_MultipleTimes_NonOverlappingFileHandles() {
	var n int
	var err error

	// Open the file.
	f, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_WRONLY, 0)
	AssertEq(nil, err)

	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	// Write some contents to the file.
	n, err = f.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	// At this point, no flushes or fsyncs should have happened.
	AssertThat(t.getFlushes(), ElementsAre())
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the file.
	err = f.Close()
	f = nil
	AssertEq(nil, err)

	// Now we should have received the flush operation (but still no fsync).
	AssertThat(t.getFlushes(), ElementsAre("taco"))
	AssertThat(t.getFsyncs(), ElementsAre())

	// Open the file again.
	f, err = os.OpenFile(path.Join(t.Dir, "foo"), os.O_WRONLY, 0)
	AssertEq(nil, err)

	// Write again; expect no further flushes.
	n, err = f.Write([]byte("p"))
	AssertEq(nil, err)
	AssertEq(1, n)

	AssertThat(t.getFlushes(), ElementsAre("taco"))
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the file. Now the new contents should be flushed.
	err = f.Close()
	f = nil
	AssertEq(nil, err)

	AssertThat(t.getFlushes(), ElementsAre("taco", "paco"))
	AssertThat(t.getFsyncs(), ElementsAre())
}

func (t *FlushFSTest) CloseReports_MultipleTimes_OverlappingFileHandles() {
	var n int
	var err error

	// Open the file with two handles.
	f1, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_WRONLY, 0)
	AssertEq(nil, err)

	f2, err := os.OpenFile(path.Join(t.Dir, "foo"), os.O_WRONLY, 0)
	AssertEq(nil, err)

	defer func() {
		if f1 != nil {
			ExpectEq(nil, f1.Close())
		}

		if f2 != nil {
			ExpectEq(nil, f2.Close())
		}
	}()

	// Write some contents with each handle.
	n, err = f1.Write([]byte("taco"))
	AssertEq(nil, err)
	AssertEq(4, n)

	n, err = f2.Write([]byte("p"))
	AssertEq(nil, err)
	AssertEq(1, n)

	// At this point, no flushes or fsyncs should have happened.
	AssertThat(t.getFlushes(), ElementsAre())
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close one handle. The current contents should be flushed.
	err = f1.Close()
	f1 = nil
	AssertEq(nil, err)

	AssertThat(t.getFlushes(), ElementsAre("paco"))
	AssertThat(t.getFsyncs(), ElementsAre())

	// Write some more contents via the other handle. Again, no further flushes.
	n, err = f2.Write([]byte("orp"))
	AssertEq(nil, err)
	AssertEq(3, n)

	AssertThat(t.getFlushes(), ElementsAre("paco"))
	AssertThat(t.getFsyncs(), ElementsAre())

	// Close the handle. Now the new contents should be flushed.
	err = f2.Close()
	f2 = nil
	AssertEq(nil, err)

	AssertThat(t.getFlushes(), ElementsAre("paco", "porp"))
	AssertThat(t.getFsyncs(), ElementsAre())
}

func (t *FlushFSTest) CloseError() {
	AssertTrue(false, "TODO")
}

func (t *FlushFSTest) FsyncReports() {
	AssertTrue(false, "TODO")
}

func (t *FlushFSTest) FsyncError() {
	AssertTrue(false, "TODO")
}
