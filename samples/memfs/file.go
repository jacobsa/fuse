// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package memfs

import (
	"sync"

	"github.com/jacobsa/fuse"
)

type memFile struct {
	/////////////////////////
	// Constant data
	/////////////////////////

	inode fuse.InodeID

	/////////////////////////
	// Mutable state
	/////////////////////////

	mu sync.RWMutex

	// The current contents of the file.
	contents []byte // GUARDED_BY(mu)
}

// TODO(jacobsa): Add a test that various WriteAt calls with a real on-disk
// file to verify what the behavior should be here, particularly when starting
// a write well beyond EOF. Leave the test around for documentation purposes.
func (f *memFile) WriteAt(p []byte, off int64) (n int, err error)
