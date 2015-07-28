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

package buffer

import (
	"unsafe"

	"github.com/jacobsa/fuse/internal/fusekernel"
)

// We size out messages to be large enough to hold a header for the response
// plus the largest read that may come in.
const outMessageSize = unsafe.Sizeof(fusekernel.OutHeader{}) + MaxReadSize

// OutMessage provides a mechanism for constructing a single contiguous fuse
// message from multiple segments, where the first segment is always a
// fusekernel.OutHeader message.
//
// Must be initialized with Reset.
type OutMessage struct {
	offset  uintptr
	storage [outMessageSize]byte
}

// Reset the message so that it is ready to be used again. Afterward, the
// contents are solely a zeroed header.
func (m *OutMessage) Reset() {
	panic("TODO")
}

// Return a pointer to the header at the start of the message.
func (b *OutMessage) OutHeader() (h *fusekernel.OutHeader) {
	panic("TODO")
}

// Grow the buffer by the supplied number of bytes, returning a pointer to the
// start of the new segment, which is zeroed. If there is no space left, return
// the nil pointer.
func (b *OutMessage) Grow(size uintptr) (p unsafe.Pointer) {
	panic("TODO")
}

// Equivalent to Grow, except the new segment is not zeroed. Use with caution!
func (b *OutMessage) GrowNoZero(size uintptr) (p unsafe.Pointer) {
	panic("TODO")
}

// Equivalent to growing by the length of p, then copying p over the new
// segment.
func (b *OutMessage) Append(p []byte) {
	panic("TODO")
}

// Equivalent to growing by the length of s, then copying s over the new
// segment.
func (b *OutMessage) AppendString(s string) {
	panic("TODO")
}

// Return the current size of the buffer.
func (b *OutMessage) Len() int {
	panic("TODO")
}

// Return a reference to the current contents of the buffer.
func (b *OutMessage) Bytes() []byte {
	panic("TODO")
}
