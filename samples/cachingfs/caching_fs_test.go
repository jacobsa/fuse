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

package cachingfs_test

import (
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/cachingfs"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

func TestHelloFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type CachingFSTest struct {
	mfs *fuse.MountedFileSystem
}

var _ TearDownInterface = &CachingFSTest{}

func init() { RegisterTestSuite(&CachingFSTest{}) }

func (t *CachingFSTest) setUp(
	lookupEntryTimeout time.Duration,
	getattrTimeout time.Duration) {
	var err error

	// Set up a temporary directory for mounting.
	mountPoint, err := ioutil.TempDir("", "caching_fs_test")
	AssertEq(nil, err)

	// Create a file system.
	fs, err := cachingfs.NewCachingFS(lookupEntryTimeout, getattrTimeout)
	AssertEq(nil, err)

	// Mount it.
	t.mfs, err = fuse.Mount(mountPoint, fs)
	AssertEq(nil, err)

	err = t.mfs.WaitForReady(context.Background())
	AssertEq(nil, err)
}

func (t *HelloFSTest) TearDown() {
	// Unmount the file system. Try again on "resource busy" errors.
	delay := 10 * time.Millisecond
	for {
		err := t.mfs.Unmount()
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "resource busy") {
			log.Println("Resource busy error while unmounting; trying again")
			time.Sleep(delay)
			delay = time.Duration(1.3 * float64(delay))
			continue
		}

		panic("MountedFileSystem.Unmount: " + err.Error())
	}

	if err := t.mfs.Join(context.Background()); err != nil {
		panic("MountedFileSystem.Join: " + err.Error())
	}
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *CachingFSTest) DoesFoo() {
	AssertTrue(false, "TODO")
}
