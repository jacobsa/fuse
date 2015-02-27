// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package fuseutil

import (
	bazilfuse "bazil.org/fuse"
	"github.com/jacobsa/fuse"
)

type DirentType bazilfuse.DirentType

// A struct representing an entry within a directory file, describing a child.
// See notes on fuse.ReadDirResponse and on AppendDirent for details.
type Dirent struct {
	// The (opaque) offset within the directory file of this entry. See notes on
	// fuse.ReadDirRequest.Offset for details.
	Offset fuse.DirOffset

	// The inode of the child file or directory, and its name within the parent.
	Inode fuse.InodeID
	Name  string

	// The type of the child. The zero value (DT_Unknown) is legal, but means
	// that the kernel will need to call GetAttr when the type is needed.
	Type DirentType
}

// Append the supplied directory entry to the given buffer in the format
// expected in fuse.ReadResponse.Data, returning the resulting buffer.
func AppendDirent(buf []byte, d Dirent) []byte {
	// We want to append bytes with the layout of fuse_dirent
	// (http://goo.gl/BmFxob) in host order. Its layout is reproduced here for
	// documentation purposes:
	type fuse_dirent struct {
		ino     uint64
		off     uint64
		namelen uint32
		type_   uint32
		name    [0]byte
	}
}
