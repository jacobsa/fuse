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

package interruptfs

import (
	"fmt"
	"os"
	"sync"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

var rootAttrs = fuseops.InodeAttributes{
	Nlink: 1,
	Mode:  os.ModeDir | 0777,
}

const fooID = fuseops.RootInodeID + 1

var fooAttrs = fuseops.InodeAttributes{
	Nlink: 1,
	Mode:  0777,
}

// A file system containing exactly one file, named "foo". Reads to the file
// always hang until interrupted. Exposes a method for synchronizing with the
// arrival of a read.
//
// Must be created with New.
type InterruptFS struct {
	fuseutil.NotImplementedFileSystem

	mu                  sync.Mutex
	readInFlight        bool
	readInFlightChanged sync.Cond
}

func New() (fs *InterruptFS) {
	fs = &InterruptFS{}
	fs.readInFlightChanged.L = &fs.mu

	return
}

////////////////////////////////////////////////////////////////////////
// Public interface
////////////////////////////////////////////////////////////////////////

// Block until the first read is received.
//
// LOCKS_EXCLUDED(fs.mu)
func (fs *InterruptFS) WaitForReadInFlight() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	for !fs.readInFlight {
		fs.readInFlightChanged.Wait()
	}
}

////////////////////////////////////////////////////////////////////////
// FileSystem methods
////////////////////////////////////////////////////////////////////////

func (fs *InterruptFS) Init(
	op *fuseops.InitOp) {
	var err error
	defer fuseutil.RespondToOp(op, &err)

	return
}

func (fs *InterruptFS) LookUpInode(
	op *fuseops.LookUpInodeOp) {
	var err error
	defer fuseutil.RespondToOp(op, &err)

	// We support only one parent.
	if op.Parent != fuseops.RootInodeID {
		err = fmt.Errorf("Unexpected parent: %v", op.Parent)
		return
	}

	// We support only one name.
	if op.Name != "foo" {
		err = fuse.ENOENT
		return
	}

	// Fill in the response.
	op.Entry.Child = fooID
	op.Entry.Attributes = fooAttrs

	return
}

func (fs *InterruptFS) GetInodeAttributes(
	op *fuseops.GetInodeAttributesOp) {
	var err error
	defer fuseutil.RespondToOp(op, &err)

	switch op.Inode {
	case fuseops.RootInodeID:
		op.Attributes = rootAttrs

	case fooID:
		op.Attributes = fooAttrs

	default:
		err = fmt.Errorf("Unexpected inode ID: %v", op.Inode)
		return
	}

	return
}

func (fs *InterruptFS) OpenFile(
	op *fuseops.OpenFileOp) {
	var err error
	defer fuseutil.RespondToOp(op, &err)

	return
}
