// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"fmt"
	"reflect"
)

// Common attributes for files and directories.
//
// TODO(jacobsa): Add tests for interacting with a file/directory after it has
// been unlinked, including creating a new file. Make sure we don't screw up
// and reuse the inode while it is still in use.
type inode struct {
	// The *memFile or *memDir for this inode, or nil if the inode is available
	// for reuse.
	//
	// INVARIANT: impl is nil, or of type *memFile or *memDir
	impl interface{}
}

func (inode *inode) checkInvariants() {
	switch inode.impl.(type) {
	case nil:
	case *memFile:
	case *memDir:
	default:
		panic(
			fmt.Sprintf("Unexpected inode impl type: %v", reflect.TypeOf(inode.impl)))
	}
}
