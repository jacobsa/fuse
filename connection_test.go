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
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/internal/buffer"
	"github.com/jacobsa/fuse/internal/fusekernel"
)

// A nil *os.PathError panics when errors.As or errors.Is unwraps it.
var nilPathErr error = (*os.PathError)(nil)

// newReplyTestConnection returns a Connection that reads one unknown op from
// a pipe. Its reply writes return writeErr.
func newReplyTestConnection(t *testing.T, writeErr error) *Connection {
	t.Helper()

	message := make([]byte, unsafe.Sizeof(fusekernel.InHeader{}))
	binary.NativeEndian.PutUint32(message[0:4], uint32(len(message)))
	binary.NativeEndian.PutUint64(message[8:16], 7)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	if _, err := w.Write(message); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()

	oldWrite := syscallWrite
	syscallWrite = func(fd int, p []byte) (int, error) {
		if writeErr != nil {
			return 0, writeErr
		}
		return len(p), nil
	}
	t.Cleanup(func() { syscallWrite = oldWrite })

	return &Connection{
		cfg:              MountConfig{OpContext: context.Background()},
		dev:              r,
		cancelFuncs:      make(map[uint64]func()),
		inMessageBufSize: buffer.GetPageSize(),
	}
}

func TestKernelResponseKeepsWrappedErrno(t *testing.T) {
	tests := []struct {
		name  string
		opErr error
		want  syscall.Errno
	}{
		{"bare errno", syscall.ENOENT, syscall.ENOENT},
		{"wrapped errno", fmt.Errorf("lookup: %w", syscall.ENOENT), syscall.ENOENT},
		{"path error", &os.PathError{Op: "open", Path: "f", Err: syscall.EACCES}, syscall.EACCES},
		{"no errno", errors.New("lookup failed"), syscall.EIO},
		{"EIO before ENOENT", fmt.Errorf("lookup: %w (cleanup: %w)", syscall.EIO, syscall.ENOENT), syscall.EIO},
		{"joined EACCES and ENOENT", errors.Join(syscall.EACCES, syscall.ENOENT), syscall.EACCES},
		{"nil path error", nilPathErr, syscall.EIO},
		{"wrapped nil path error", fmt.Errorf("lookup: %w", nilPathErr), syscall.EIO},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Connection
			var m buffer.OutMessage
			m.Reset()
			c.kernelResponse(&m, 1, &fuseops.LookUpInodeOp{}, tt.opErr)
			if got := m.OutHeader().Error; got != -int32(tt.want) {
				t.Errorf("reply error = %d, want %d", got, -int32(tt.want))
			}
		})
	}
}

func TestShouldLogErrorMatchesWrappedErrno(t *testing.T) {
	wrap := func(errno syscall.Errno) error { return fmt.Errorf("op: %w", errno) }
	tests := []struct {
		name string
		op   interface{}
		err  error
		want bool
	}{
		{"lookup ENOENT", &fuseops.LookUpInodeOp{}, wrap(syscall.ENOENT), false},
		{"getxattr ENODATA", &fuseops.GetXattrOp{}, wrap(syscall.ENODATA), false},
		{"listxattr ERANGE", &fuseops.ListXattrOp{}, wrap(syscall.ERANGE), false},
		{"getxattr ENOSYS", &fuseops.GetXattrOp{}, wrap(syscall.ENOSYS), false},
		{"unknown op ENOSYS", &unknownOp{}, wrap(syscall.ENOSYS), false},
		{"lookup EIO", &fuseops.LookUpInodeOp{}, wrap(syscall.EIO), true},
		{"lookup EIO before ENOENT", &fuseops.LookUpInodeOp{}, fmt.Errorf("lookup: %w (cleanup: %w)", syscall.EIO, syscall.ENOENT), true},
		{"lookup joined EACCES and ENOENT", &fuseops.LookUpInodeOp{}, errors.Join(syscall.EACCES, syscall.ENOENT), true},
		{"lookup no errno", &fuseops.LookUpInodeOp{}, errors.New("lookup failed"), true},
		{"lookup nil path error", &fuseops.LookUpInodeOp{}, nilPathErr, true},
		{"lookup wrapped nil path error", &fuseops.LookUpInodeOp{}, fmt.Errorf("lookup: %w", nilPathErr), true},
	}
	c := &Connection{errorLogger: log.New(io.Discard, "", 0)}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.shouldLogError(tt.op, tt.err); got != tt.want {
				t.Errorf("shouldLogError = %v, want %v", got, tt.want)
			}
		})
	}
}

// The kernel answers a reply to a request it no longer tracks with ENOENT.
func TestReplyTreatsENOENTWriteAsBenign(t *testing.T) {
	tests := []struct {
		name     string
		writeErr error
		wantErr  bool
	}{
		{"ENOENT", syscall.ENOENT, false},
		{"EIO", syscall.EIO, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logged bytes.Buffer
			c := newReplyTestConnection(t, tt.writeErr)
			c.errorLogger = log.New(&logged, "", 0)
			ctx, _, err := c.ReadOp()
			if err != nil {
				t.Fatalf("ReadOp: %v", err)
			}

			err = c.Reply(ctx, syscall.ENOSYS)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reply = %v, want error: %v", err, tt.wantErr)
			}
			if (logged.Len() != 0) != tt.wantErr {
				t.Errorf("logged %q, want a log: %v", logged.String(), tt.wantErr)
			}
		})
	}
}

func TestFormatWireLogEntryStatus(t *testing.T) {
	tests := []struct {
		name  string
		opErr error
		want  int
	}{
		{"wrapped errno", fmt.Errorf("lookup: %w", syscall.ENOENT), int(syscall.ENOENT)},
		{"no errno", errors.New("lookup failed"), 0},
		{"nil path error", nilPathErr, 0},
		{"wrapped nil path error", fmt.Errorf("lookup: %w", nilPathErr), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wlog := NewWireLogRecord()
			if _, err := formatWireLogEntry(&fuseops.LookUpInodeOp{}, tt.opErr, wlog); err != nil {
				t.Fatalf("formatWireLogEntry: %v", err)
			}
			if wlog.Status != tt.want {
				t.Errorf("Status = %d, want %d", wlog.Status, tt.want)
			}
		})
	}
}

// fmt recovers from a panic in Error when it formats the error value itself.
func TestReplyDebugLogsOpError(t *testing.T) {
	tests := []struct {
		name        string
		opErr       error
		errorLogger bool
		want        string
	}{
		{"ordinary error", errors.New("lookup failed"), false, `-> Error: "lookup failed"`},
		{"nil path error", nilPathErr, false, "-> Error: <nil>"},
		{"suppressed errno joined with nil path error", errors.Join(syscall.ENOSYS, nilPathErr), true,
			"-> Error: %!q(PANIC=Error method: runtime error: invalid memory address or nil pointer dereference)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var debugged bytes.Buffer
			c := newReplyTestConnection(t, nil)
			c.debugLogger = log.New(&debugged, "", 0)
			if tt.errorLogger {
				c.errorLogger = log.New(io.Discard, "", 0)
			}
			ctx, _, err := c.ReadOp()
			if err != nil {
				t.Fatalf("ReadOp: %v", err)
			}

			if err := c.Reply(ctx, tt.opErr); err != nil {
				t.Errorf("Reply: %v", err)
			}
			if !strings.HasSuffix(debugged.String(), "] "+tt.want+"\n") {
				t.Errorf("debug log = %q, want it to end with %q", debugged.String(), tt.want)
			}
		})
	}
}

func TestSanitizeMaxPagesAndWrite(t *testing.T) {
	pageSize := uint32(buffer.GetPageSize())
	defaultMaxWrite := uint32(buffer.MaxWriteSize)
	defaultMaxPages := uint16(max(uint32(buffer.MaxReadSize), uint32(buffer.MaxWriteSize)) / pageSize)

	tests := []struct {
		name       string
		inputPages uint16
		inputWrite uint32
		wantPages  uint16
		wantWrite  uint32
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "neither given",
			inputPages: 0,
			inputWrite: 0,
			wantPages:  defaultMaxPages,
			wantWrite:  defaultMaxWrite,
			wantErr:    false,
		},
		{
			name:       "only MaxWrite given (128KB)",
			inputPages: 0,
			inputWrite: 128 * 1024,
			wantPages:  defaultMaxPages, // max(128KB, 1MB) / 4KB
			wantWrite:  128 * 1024,
			wantErr:    false,
		},
		{
			name:       "only MaxWrite given (not page aligned, 129KB)",
			inputPages: 0,
			inputWrite: 129 * 1024,
			wantPages:  defaultMaxPages, // max(129KB, 1MB) / 4KB
			wantWrite:  129 * 1024,
			wantErr:    false,
		},
		{
			name:       "only MaxWrite given (larger than 1MB, e.g. 2MB)",
			inputPages: 0,
			inputWrite: 2 * 1024 * 1024,
			wantPages:  512, // 2MB / 4KB
			wantWrite:  2 * 1024 * 1024,
			wantErr:    false,
		},
		{
			name:       "only MaxPages given (valid, e.g. 256)",
			inputPages: 256,
			inputWrite: 0,
			wantPages:  256,
			wantWrite:  defaultMaxWrite, // 256 * 4KB
			wantErr:    false,
		},
		{
			name:       "only MaxPages given (small, e.g. 32)",
			inputPages: 32,
			inputWrite: 0,
			wantPages:  32,
			wantWrite:  128 * 1024, // 32 * 4KB
			wantErr:    false,
		},
		{
			name:       "only MaxPages given (very small, e.g. 1)",
			inputPages: 1,
			inputWrite: 0,
			wantPages:  1,
			wantWrite:  4096, // 1 * 4KB
			wantErr:    false,
		},
		{
			name:       "both given (valid, e.g. MaxPages=32, MaxWrite=128KB)",
			inputPages: 32,
			inputWrite: 128 * 1024,
			wantPages:  32,
			wantWrite:  128 * 1024,
			wantErr:    false,
		},
		{
			name:       "both given (invalid, MaxPages=16, MaxWrite=128KB)",
			inputPages: 16,
			inputWrite: 128 * 1024,
			wantPages:  16,
			wantWrite:  128 * 1024,
			wantErr:    true,
			errSubstr:  "must be at least MaxWrite",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Connection{
				cfg: MountConfig{
					MaxPages: tc.inputPages,
					MaxWrite: tc.inputWrite,
				},
			}

			err := c.sanitizeMaxPagesAndWrite()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("expected error to contain %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if c.cfg.MaxPages != tc.wantPages {
					t.Errorf("MaxPages = %d, want %d", c.cfg.MaxPages, tc.wantPages)
				}
				if c.cfg.MaxWrite != tc.wantWrite {
					t.Errorf("MaxWrite = %d, want %d", c.cfg.MaxWrite, tc.wantWrite)
				}
			}
		})
	}
}
