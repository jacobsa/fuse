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
	"errors"
	"io"
	"unsafe"

	"github.com/jacobsa/fuse/internal/fusekernel"
)

// An incoming message from the kernel, including leading fusekernel.InHeader
// struct. Provides storage for messages and convenient access to their
// contents.
type InMessage struct {
}

// Initialize with the data read by a single call to r.Read. The first call to
// Consume will consume the bytes directly after the fusekernel.InHeader
// struct.
func (m *InMessage) Init(r io.Reader) (err error) {
	err = errors.New("TODO")
	return
}

// Return a reference to the header read in the most recent call to Init.
func (m *InMessage) Header() (h *fusekernel.InHeader) {
	panic("TODO")
}

// Consume the next n bytes from the message, returning a nil pointer if there
// are fewer than n bytes available.
func (m *InMessage) Consume(n uintptr) (p unsafe.Pointer) {
	panic("TODO")
}

// Equivalent to Consume, except returns a slice of bytes. The result will be
// nil if Consume fails.
func (m *InMessage) ConsumeBytes(n uintptr) (b []byte) {
	panic("TODO")
}
