// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"fmt"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
)

type memDir struct {
	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The contents of the directory. An entry with inode zero is unused.
	//
	// This array can never be shortened, nor can its elements be moved, because
	// we use its indices for Dirent.Offset, which is exposed to the user who
	// might be calling readdir in a loop while concurrently modifying the
	// directory. Unused entries can, however, be reused.
	//
	// TODO(jacobsa): Add good tests exercising concurrent modifications while
	// doing readdir, seekdir, etc. calls.
	//
	// INVARIANT: For each i, entries[i].Offset == i+1
	entries []fuseutil.Dirent
}

func newDir() (d *memDir) {
	d = &memDir{}
	d.mu = syncutil.NewInvariantMutex(d.checkInvariants)

	return
}

func (d *memDir) checkInvariants() {
	for i, e := range d.entries {
		if e.Offset != fuse.DirOffset(i+1) {
			panic(fmt.Sprintf("Unexpected offset in entry: %v", e))
		}
	}
}

// Find the inode ID of the child with the given name.
//
// LOCKS_EXCLUDED(d.mu)
func (d *memDir) LookUpInode(name string) (id fuse.InodeID, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, e := range d.entries {
		if e.Name == name {
			ok = true
			id = e.Inode
			return
		}
	}

	return
}
