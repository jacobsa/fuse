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
	"sync"
	"unsafe"

	"github.com/jacobsa/fuse/internal/freelist"
)

// MessageProvider is used to get and release the buffers
// needed to communicate with the kernel. Any implementations
// of this interface must be thread safe.
type MessageProvider interface {
	// GetInMessage is called before reading each operation from the
	// kernel. It is generally recommended to maintain a pool of
	// InMessages rather than generating a new InMessage for every
	// operation, since the buffer size needed to read an operation
	// from the kernel can be quite large. A new InMessage can be
	// generated with NewInMessage.
	GetInMessage() *InMessage

	// GetOutMessage is called after reading each operation from the
	// kernel. Reset should be called on any OutMessage before it can
	// be safely reused.
	GetOutMessage() *OutMessage

	// PutInMessage and PutOutMessage are called either on error, or after
	// the response to a FUSE operation has been written to the kernel.
	PutInMessage(*InMessage)
	PutOutMessage(*OutMessage)
}

// DefaultMessageProvider is used as the implementation for MessageProvider
// if no custom implementation is provided in the MountConfig. This class
// uses a simple list of pointers to recycle InMessages and OutMessages.
type DefaultMessageProvider struct {
	mu sync.Mutex

	inMessages  freelist.Freelist // GUARDED_BY(mu)
	outMessages freelist.Freelist // GUARDED_BY(mu)
}

func (m *DefaultMessageProvider) GetInMessage() *InMessage {
	m.mu.Lock()
	x := (*InMessage)(m.inMessages.Get())
	m.mu.Unlock()

	if x == nil {
		x = NewInMessage()
	}

	return x
}

func (m *DefaultMessageProvider) GetOutMessage() *OutMessage {
	m.mu.Lock()
	x := (*OutMessage)(m.outMessages.Get())
	m.mu.Unlock()

	if x == nil {
		x = new(OutMessage)
	}
	x.Reset()

	return x
}

func (m *DefaultMessageProvider) PutInMessage(x *InMessage) {
	m.mu.Lock()
	m.inMessages.Put(unsafe.Pointer(x))
	m.mu.Unlock()
}

func (m *DefaultMessageProvider) PutOutMessage(x *OutMessage) {
	m.mu.Lock()
	m.outMessages.Put(unsafe.Pointer(x))
	m.mu.Unlock()
}
