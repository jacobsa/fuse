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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacobsa/fuse/internal/buffer"
)

func TestMalformedMessageError(t *testing.T) {
	inner := errors.New("Corrupt OpLookup")
	msg := []byte{0x00, 0x01, 0x02, 0xff}
	err := &MalformedMessageError{Err: inner, Message: msg}

	if got, want := err.Error(), "convertInMessage: Corrupt OpLookup"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// The error unwraps to the underlying conversion error and is recoverable
	// via errors.As even when wrapped.
	if !errors.Is(err, inner) {
		t.Errorf("errors.Is(err, inner) = false, want true")
	}

	var target *MalformedMessageError
	if !errors.As(fmt.Errorf("read op: %w", err), &target) {
		t.Fatalf("errors.As failed to recover *MalformedMessageError")
	}
	if !bytes.Equal(target.Message, msg) {
		t.Errorf("Message = %v, want %v", target.Message, msg)
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
