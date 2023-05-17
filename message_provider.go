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

package fuse

import (
	"sync"
	"unsafe"

	"github.com/jacobsa/fuse/internal/freelist"
)

type MessageProvider interface {
	GetInMessage() *InMessage

	GetOutMessage() *OutMessage

	PutInMessage(*InMessage)

	PutOutMessage(*OutMessage)
}

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
