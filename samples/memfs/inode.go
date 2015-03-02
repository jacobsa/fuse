// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/gcloud/syncutil"
)

// Common attributes for files and directories.
//
// TODO(jacobsa): Add tests for interacting with a file/directory after it has
// been unlinked, including creating a new file. Make sure we don't screw up
// and reuse an inode ID while it is still in use.
type inode struct {
	/////////////////////////
	// Constant data
	/////////////////////////

	// Is this a directory? If not, it is a file.
	dir bool

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu syncutil.InvariantMutex

	// The current attributes of this inode.
	//
	// INVARIANT: No non-permission mode bits are set besides os.ModeDir
	// INVARIANT: If dir, then os.ModeDir is set
	// INVARIANT: If !dir, then os.ModeDir is not set
	attributes fuse.InodeAttributes // GUARDED_BY(mu)

	// For directories, entries describing the children of the directory.
	//
	// This array can never be shortened, nor can its elements be moved, because
	// we use its indices for Dirent.Offset, which is exposed to the user who
	// might be calling readdir in a loop while concurrently modifying the
	// directory. Unused entries can, however, be reused.
	//
	// TODO(jacobsa): Add good tests exercising concurrent modifications while
	// doing readdir, seekdir, etc. calls.
	//
	// INVARIANT: If dir is false, this is nil.
	// INVARIANT: For each i, entries[i].Offset == i+1
	entries []fuseutil.Dirent // GUARDED_BY(mu)

	// For files, the current contents of the file.
	//
	// INVARIANT: If dir is true, this is nil.
	contents []byte // GUARDED_BY(mu)
}

func newInode(dir bool) (inode *inode)

func (inode *inode) checkInvariants()

// Find an entry for the given child name and return its inode ID.
//
// REQUIRES: inode.dir
// SHARED_LOCKS_REQUIRED(inode.mu)
func (inode *inode) LookUpChild(name string) (id fuse.InodeID, ok bool)
