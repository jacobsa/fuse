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

package fuseutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"
	"testing"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
)

type forgetRecorder struct {
	NotImplementedFileSystem
	batchErr  error
	forgotten []fuseops.InodeID
}

func (fs *forgetRecorder) BatchForget(context.Context, *fuseops.BatchForgetOp) error {
	return fs.batchErr
}

func (fs *forgetRecorder) ForgetInode(_ context.Context, op *fuseops.ForgetInodeOp) error {
	fs.forgotten = append(fs.forgotten, op.Inode)
	return nil
}

func TestBatchForgetFallsBackToForgetInodeOnENOSYS(t *testing.T) {
	eioBeforeENOSYS := fmt.Errorf("batch forget: %w (cleanup: %w)", syscall.EIO, fuse.ENOSYS)
	// A nil *os.PathError panics when errors.As or errors.Is unwraps it.
	var nilPathErr error = (*os.PathError)(nil)
	wrappedNilPathErr := fmt.Errorf("batch forget: %w", nilPathErr)
	tests := []struct {
		name     string
		batchErr error
		wantErr  error
		want     []fuseops.InodeID
	}{
		{"bare ENOSYS", fuse.ENOSYS, nil, []fuseops.InodeID{2, 3}},
		{"wrapped ENOSYS", fmt.Errorf("batch forget: %w", fuse.ENOSYS), nil, []fuseops.InodeID{2, 3}},
		{"implemented", nil, nil, nil},
		{"other error", syscall.EIO, syscall.EIO, nil},
		{"EIO before ENOSYS", eioBeforeENOSYS, eioBeforeENOSYS, nil},
		{"nil path error", nilPathErr, nilPathErr, nil},
		{"wrapped nil path error", wrappedNilPathErr, wrappedNilPathErr, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &forgetRecorder{batchErr: tt.batchErr}
			s := &fileSystemServer{fs: fs}
			op := &fuseops.BatchForgetOp{Entries: []fuseops.BatchForgetEntry{{Inode: 2, N: 1}, {Inode: 3, N: 1}}}
			if err := s.batchForget(context.Background(), op); !errors.Is(err, tt.wantErr) {
				t.Errorf("batchForget = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(fs.forgotten, tt.want) {
				t.Errorf("forgotten inodes = %v, want %v", fs.forgotten, tt.want)
			}
		})
	}
}
