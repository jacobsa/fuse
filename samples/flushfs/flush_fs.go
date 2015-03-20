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

package flushfs

import (
	"os"
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"golang.org/x/net/context"
)

// Create a file system containing a single file named "foo".
//
// The file may be opened for reading and/or writing. Its initial contents are
// empty. Whenever a flush or fsync is received, the supplied function will be
// called with the current contents of the file.
func NewFileSystem(
	reportFlush func(string),
	reportFsync func(string)) (fs fuse.FileSystem, err error) {
	fs = &flushFS{}
	return
}

const fooID = fuse.RootInodeID + 1

type flushFS struct {
	fuseutil.NotImplementedFileSystem

	mu  sync.Mutex
	foo *os.File // GUARDED_BY(mu)
}

////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) rootAttributes() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Nlink: 1,
		Mode:  0777 | os.ModeDir,
	}
}

// LOCKS_REQUIRED(fs.mu)
func (fs *flushFS) fooAttributes() fuse.InodeAttributes {
	return fuse.InodeAttributes{
		Nlink: 1,
		Mode:  0777,
	}
}

////////////////////////////////////////////////////////////////////////
// File system methods
////////////////////////////////////////////////////////////////////////

func (fs *flushFS) Init(
	ctx context.Context,
	req *fuse.InitRequest) (
	resp *fuse.InitResponse, err error) {
	resp = &fuse.InitResponse{}
	return
}

func (fs *flushFS) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (
	resp *fuse.LookUpInodeResponse, err error) {
	resp = &fuse.LookUpInodeResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Sanity check.
	if req.Parent != fuse.RootInodeID || req.Name != "foo" {
		err = fuse.ENOENT
		return
	}

	resp.Entry = fuse.ChildInodeEntry{
		Child:      fooID,
		Attributes: fs.fooAttributes(),
	}

	return
}

func (fs *flushFS) GetInodeAttributes(
	ctx context.Context,
	req *fuse.GetInodeAttributesRequest) (
	resp *fuse.GetInodeAttributesResponse, err error) {
	resp = &fuse.GetInodeAttributesResponse{}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch req.Inode {
	case fuse.RootInodeID:
		resp.Attributes = fs.rootAttributes()
		return

	case fooID:
		resp.Attributes = fs.fooAttributes()
		return

	default:
		err = fuse.ENOENT
		return
	}
}
