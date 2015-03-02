// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

// Common attributes for files and directories.
type inode struct {
	// The *memFile or *memDir for this inode, or nil if the inode is available
	// for reuse.
	//
	// INVARIANT: impl is nil, or of type *memFile or *memDir
	impl interface{}
}
