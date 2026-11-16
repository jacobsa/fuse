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

package fuse

import (
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse/internal/buffer"
)

// fuse_dev_do_read fails with ECONNABORTED when the connection is aborted
// after the read took a request off the pending list. An unmount aborts the
// connection.
func TestReadOpReturnsEOFWhenTheReadIsAborted(t *testing.T) {
	tests := []struct {
		name    string
		errno   syscall.Errno
		wantEOF bool
	}{
		{"ENODEV", syscall.ENODEV, true},
		{"ECONNABORTED", syscall.ECONNABORTED, true},
		{"EIO", syscall.EIO, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readErr error = &os.PathError{Op: "read", Path: "/dev/fuse", Err: tt.errno}
			oldInit := initInMessage
			initInMessage = func(*buffer.InMessage, io.Reader) error { return readErr }
			t.Cleanup(func() { initInMessage = oldInit })

			c := &Connection{inMessageBufSize: buffer.GetPageSize()}
			_, _, err := c.ReadOp()
			if tt.wantEOF && err != io.EOF {
				t.Errorf("ReadOp = %v, want io.EOF", err)
			}
			if !tt.wantEOF && err != readErr {
				t.Errorf("ReadOp = %v, want %v", err, readErr)
			}
		})
	}
}
