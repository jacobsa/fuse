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
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// These tests run the test binary as the mount helper that fusermount starts.
// None of them mounts.

// fakeMountHelper makes TestFakeMountHelper act as a mount helper. Its value
// says what the helper does:
//
//   - "exit": exit with status 0 without passing a device.
//   - "pass:<fds>": exit with status 1 when the helper inherited a socket other
//     than fd 3, its end of the socket pair, and the comma-separated <fds>.
//     Otherwise pass /dev/null back as the device and exit with status 0.
const fakeMountHelper = "FUSE_TEST_FAKE_MOUNT_HELPER"

// runFakeMountHelper runs fusermount with TestFakeMountHelper as the helper.
func runFakeMountHelper(mode string, wait bool) (*os.File, error) {
	argv := []string{"-test.run=^TestFakeMountHelper$", "-test.timeout=1m"}
	return fusermount(os.Args[0], argv, []string{fakeMountHelper + "=" + mode}, wait, nil)
}

// openSockets returns the open file descriptors above 3 that are sockets.
func openSockets() ([]string, error) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return nil, err
	}
	var sockets []string
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd <= 3 {
			continue
		}
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) == nil && st.Mode&syscall.S_IFMT == syscall.S_IFSOCK {
			sockets = append(sockets, entry.Name())
		}
	}
	return sockets, nil
}

func TestFakeMountHelper(t *testing.T) {
	mode, ok := os.LookupEnv(fakeMountHelper)
	if !ok {
		t.Skip("runs only as the mount helper that fusermount starts")
	}
	if mode == "exit" {
		os.Exit(0)
	}
	held, ok := strings.CutPrefix(mode, "pass:")
	if !ok {
		os.Exit(2)
	}
	sockets, err := openSockets()
	if err != nil {
		os.Exit(2)
	}
	for _, fd := range sockets {
		if !slices.Contains(strings.Split(held, ","), fd) {
			fmt.Fprintf(os.Stderr, "the mount helper inherited socket %s\n", fd)
			os.Exit(1)
		}
	}
	dev, err := os.Open(os.DevNull)
	if err != nil {
		os.Exit(2)
	}
	if err := syscall.Sendmsg(3, []byte{0}, syscall.UnixRights(int(dev.Fd())), nil, 0); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// A helper that exits without passing a device makes fusermount fail.
func TestFusermountFailsWhenTheHelperPassesNoDevice(t *testing.T) {
	for _, wait := range []bool{true, false} {
		t.Run(fmt.Sprintf("wait=%v", wait), func(t *testing.T) {
			dev, err := runFakeMountHelper("exit", wait)
			if err == nil {
				dev.Close()
				t.Fatal("fusermount succeeded, want an error")
			}
			// Depending on the Go version, ReadMsgUnix returns io.EOF bare or in
			// a *net.OpError.
			if got := err.Error(); !strings.HasPrefix(got, "ReadMsgUnix: ") || !strings.HasSuffix(got, ": EOF") {
				t.Errorf("fusermount error = %q, want ReadMsgUnix to fail with EOF", got)
			}
		})
	}
}

// The helper inherits only fd 3 of the socket pair. A process that holds
// another copy of the helper's end keeps fusermount's read from ending when
// the helper exits without passing a device.
func TestMountHelperInheritsOnlyItsEndOfTheSocket(t *testing.T) {
	// The test process can hold sockets it inherited, as on some CI runners.
	// The helper inherits those too, so it ignores them.
	held, err := openSockets()
	if err != nil {
		t.Fatalf("listing open sockets: %v", err)
	}
	dev, err := runFakeMountHelper("pass:"+strings.Join(held, ","), true)
	if err != nil {
		t.Fatalf("fusermount: %v", err)
	}
	dev.Close()
}
