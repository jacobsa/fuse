// Copyright 2025 Google Inc. All Rights Reserved.
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

package fusekernel

import "testing"

// TestKillPrivFlagValues verifies that our flag values match libfuse
// These values are from https://github.com/libfuse/libfuse/blob/master/include/fuse_kernel.h
func TestKillPrivFlagValues(t *testing.T) {
	tests := []struct {
		name     string
		flag     uint32
		expected uint32
	}{
		// Init flags
		{"InitHandleKillpriv", uint32(InitHandleKillpriv), 1 << 19},
		{"InitHandleKillprivV2", uint32(InitHandleKillprivV2), 1 << 28},

		// Setattr flag
		{"SetattrKillSuidgid", uint32(SetattrKillSuidgid), 1 << 11},

		// Write flag
		{"WriteKillSuidgid", uint32(WriteKillSuidgid), 1 << 2},

		// Open/Create flag
		{"OpenKillSuidgid", uint32(OpenKillSuidgid), 1 << 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.flag != tt.expected {
				t.Errorf("%s = %d (0x%x), want %d (0x%x)",
					tt.name, tt.flag, tt.flag, tt.expected, tt.expected)
			}
		})
	}
}

// TestSetattrValidKillSuidgid tests the KillSuidgid helper method
func TestSetattrValidKillSuidgid(t *testing.T) {
	tests := []struct {
		name  string
		valid SetattrValid
		want  bool
	}{
		{
			name:  "KillSuidgid set",
			valid: SetattrKillSuidgid,
			want:  true,
		},
		{
			name:  "KillSuidgid with other flags",
			valid: SetattrKillSuidgid | SetattrMode | SetattrSize,
			want:  true,
		},
		{
			name:  "KillSuidgid not set",
			valid: SetattrMode | SetattrSize,
			want:  false,
		},
		{
			name:  "No flags",
			valid: 0,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.valid.KillSuidgid(); got != tt.want {
				t.Errorf("SetattrValid.KillSuidgid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWriteFlagsKillSuidgid tests WriteFlags values
func TestWriteFlagsKillSuidgid(t *testing.T) {
	tests := []struct {
		name  string
		flags WriteFlags
		want  bool
	}{
		{
			name:  "KillSuidgid set",
			flags: WriteKillSuidgid,
			want:  true,
		},
		{
			name:  "KillSuidgid with other flags",
			flags: WriteKillSuidgid | WriteCache | WriteLockOwner,
			want:  true,
		},
		{
			name:  "KillSuidgid not set",
			flags: WriteCache | WriteLockOwner,
			want:  false,
		},
		{
			name:  "No flags",
			flags: 0,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (tt.flags & WriteKillSuidgid) != 0
			if got != tt.want {
				t.Errorf("WriteFlags & WriteKillSuidgid = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOpenRequestFlagsKillSuidgid tests OpenRequestFlags values
func TestOpenRequestFlagsKillSuidgid(t *testing.T) {
	tests := []struct {
		name  string
		flags OpenRequestFlags
		want  bool
	}{
		{
			name:  "KillSuidgid set",
			flags: OpenKillSuidgid,
			want:  true,
		},
		{
			name:  "No flags",
			flags: 0,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (tt.flags & OpenKillSuidgid) != 0
			if got != tt.want {
				t.Errorf("OpenRequestFlags & OpenKillSuidgid = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInitFlagsKillPriv tests InitFlags for KILLPRIV support
func TestInitFlagsKillPriv(t *testing.T) {
	tests := []struct {
		name      string
		flags     InitFlags
		hasV1     bool
		hasV2     bool
	}{
		{
			name:      "V1 only",
			flags:     InitHandleKillpriv,
			hasV1:     true,
			hasV2:     false,
		},
		{
			name:      "V2 only",
			flags:     InitHandleKillprivV2,
			hasV1:     false,
			hasV2:     true,
		},
		{
			name:      "Both V1 and V2",
			flags:     InitHandleKillpriv | InitHandleKillprivV2,
			hasV1:     true,
			hasV2:     true,
		},
		{
			name:      "Neither",
			flags:     InitAsyncRead | InitFileOps,
			hasV1:     false,
			hasV2:     false,
		},
		{
			name:      "No flags",
			flags:     0,
			hasV1:     false,
			hasV2:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotV1 := (tt.flags & InitHandleKillpriv) != 0
			gotV2 := (tt.flags & InitHandleKillprivV2) != 0

			if gotV1 != tt.hasV1 {
				t.Errorf("InitHandleKillpriv flag: got %v, want %v", gotV1, tt.hasV1)
			}
			if gotV2 != tt.hasV2 {
				t.Errorf("InitHandleKillprivV2 flag: got %v, want %v", gotV2, tt.hasV2)
			}
		})
	}
}

// TestKillPrivFlagStrings tests that flag names are properly registered
func TestKillPrivFlagStrings(t *testing.T) {
	tests := []struct {
		name       string
		stringer   interface{ String() string }
		wantSubstr string
	}{
		{
			name:       "InitHandleKillpriv",
			stringer:   InitHandleKillpriv,
			wantSubstr: "InitHandleKillpriv",
		},
		{
			name:       "InitHandleKillprivV2",
			stringer:   InitHandleKillprivV2,
			wantSubstr: "InitHandleKillprivV2",
		},
		{
			name:       "SetattrKillSuidgid",
			stringer:   SetattrKillSuidgid,
			wantSubstr: "SetattrKillSuidgid",
		},
		{
			name:       "WriteKillSuidgid",
			stringer:   WriteKillSuidgid,
			wantSubstr: "WriteKillSuidgid",
		},
		{
			name:       "OpenKillSuidgid",
			stringer:   OpenKillSuidgid,
			wantSubstr: "OpenKillSuidgid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stringer.String()
			if got != tt.wantSubstr {
				t.Errorf("%s.String() = %q, want %q", tt.name, got, tt.wantSubstr)
			}
		})
	}
}
