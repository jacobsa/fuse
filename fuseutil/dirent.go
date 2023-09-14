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

package fuseutil

import (
	"unsafe"

	"github.com/jacobsa/fuse/fuseops"
)

// Write the supplied directory entry into the given buffer in the format
// expected in fuseops.ReadFileOp.Data, returning the number of bytes written.
// Return zero if the entry would not fit.
func WriteDirent(buf []byte, d fuseops.Dirent) (n int) {
	// We want to write bytes with the layout of fuse_dirent
	// (http://goo.gl/BmFxob) in host order. The struct must be aligned according
	// to FUSE_DIRENT_ALIGN (http://goo.gl/UziWvH), which dictates 8-byte
	// alignment.
	type fuse_dirent struct {
		ino     uint64
		off     uint64
		namelen uint32
		type_   uint32
		name    [0]byte
	}

	const direntAlignment = 8
	const direntSize = 8 + 8 + 4 + 4

	// Compute the number of bytes of padding we'll need to maintain alignment
	// for the next entry.
	var padLen int
	if len(d.Name)%direntAlignment != 0 {
		padLen = direntAlignment - (len(d.Name) % direntAlignment)
	}

	// Do we have enough room?
	totalLen := direntSize + len(d.Name) + padLen
	if totalLen > len(buf) {
		return n
	}

	// Write the header.
	de := fuse_dirent{
		ino:     uint64(d.Inode),
		off:     uint64(d.Offset),
		namelen: uint32(len(d.Name)),
		type_:   uint32(d.Type),
	}

	n += copy(buf[n:], (*[direntSize]byte)(unsafe.Pointer(&de))[:])

	// Write the name afterward.
	n += copy(buf[n:], d.Name)

	// Add any necessary padding.
	if padLen != 0 {
		var padding [direntAlignment]byte
		n += copy(buf[n:], padding[:padLen])
	}

	return n
}
