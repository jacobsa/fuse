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
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jacobsa/fuse/internal/fusekernel"
)

// blockingServer is a Server whose ServeOps closes started, then returns once
// release is closed. It never reads the connection.
type blockingServer struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingServer() blockingServer {
	return blockingServer{started: make(chan struct{}), release: make(chan struct{})}
}

func (s blockingServer) ServeOps(*Connection) {
	close(s.started)
	<-s.release
}

func checkNotifierENOTCONN(t *testing.T, n *Notifier, when string) {
	t.Helper()
	if err := n.InvalidateInode(1, 0, 0); !errors.Is(err, syscall.ENOTCONN) {
		t.Errorf("InvalidateInode %s = %v, want ENOTCONN", when, err)
	}
	if err := n.InvalidateEntry(1, "x"); !errors.Is(err, syscall.ENOTCONN) {
		t.Errorf("InvalidateEntry %s = %v, want ENOTCONN", when, err)
	}
}

func TestNotifierReturnsENOTCONNWhileNothingServes(t *testing.T) {
	n := NewNotifier()
	checkNotifierENOTCONN(t, n, "before ServeOps")

	s := newBlockingServer()
	close(s.release)
	NewServerWithNotifier(n, s).ServeOps(nil)
	checkNotifierENOTCONN(t, n, "after ServeOps returned")
}

// While the wrapped server serves, the Notifier writes each notification to
// the connection and returns the write's result.
func TestNotifierSendsWhileServing(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	n := NewNotifier()
	s := newBlockingServer()
	served := make(chan struct{})
	go func() {
		NewServerWithNotifier(n, s).ServeOps(&Connection{dev: w})
		close(served)
	}()
	<-s.started

	if err := n.InvalidateInode(7, 4096, 8192); err != nil {
		t.Fatalf("InvalidateInode: %v", err)
	}
	var msg struct {
		Header fusekernel.OutHeader
		Body   fusekernel.NotifyInvalInodeOut
	}
	if _, err := io.ReadFull(r, unsafe.Slice((*byte)(unsafe.Pointer(&msg)), unsafe.Sizeof(msg))); err != nil {
		t.Fatalf("reading the notification: %v", err)
	}
	if msg.Header.Error != fusekernel.NotifyCodeInvalInode || msg.Header.Unique != 0 {
		t.Errorf("notification header = %+v, want error %d and unique 0", msg.Header, fusekernel.NotifyCodeInvalInode)
	}
	if want := (fusekernel.NotifyInvalInodeOut{Ino: 7, Off: 4096, Len: 8192}); msg.Body != want {
		t.Errorf("notification body = %+v, want %+v", msg.Body, want)
	}

	close(s.release)
	<-served
}
