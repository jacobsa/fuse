// Copyright 2026 Google Inc. All Rights Reserved.
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
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/jacobsa/fuse/internal/fusekernel"
)

func TestInMessageBytes(t *testing.T) {
	// Construct a well-formed message: a header whose Len field matches the
	// total size, followed by a small payload.
	const payloadLen = 8
	total := fusekernel.InHeaderSize + payloadLen
	raw := make([]byte, total)
	binary.LittleEndian.PutUint32(raw[0:4], uint32(total))
	for i := fusekernel.InHeaderSize; i < total; i++ {
		raw[i] = byte(i)
	}

	m := NewInMessage(GetPageSize() + MaxWriteSize)
	if err := m.Init(bytes.NewReader(raw)); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := m.Bytes(); !bytes.Equal(got, raw) {
		t.Errorf("Bytes() = %v, want %v", got, raw)
	}
}
