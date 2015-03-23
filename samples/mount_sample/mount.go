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

// A simple tool for mounting sample file systems, used by the tests in
// samples/.
package main

import (
	"flag"
	"log"

	"github.com/jacobsa/fuse"
	"golang.org/x/net/context"
)

var fType = flag.String("type", "", "The name of the samples/ sub-dir.")
var fMountPoint = flag.String("mount_point", "", "Path to mount point.")

var fFlushesFile = flag.String(
	"flushfs.flushes_file",
	"",
	"Path to a file to which flushes should be reported, \\n-separated.")

var fFsyncsFile = flag.String(
	"flushfs.fsyncs_file",
	"",
	"Path to a file to which fsyncs should be reported, \\n-separated.")

func makeFS() (fs fuse.FileSystem, err error)

func main() {
	flag.Parse()

	// Create an appropriate file system.
	fs, err := makeFS()
	if err != nil {
		log.Fatalf("makeFS: %v", err)
	}

	// Mount the file system.
	if *fMountPoint == "" {
		log.Fatalf("You must set --mount_point.")
	}

	mfs, err := fuse.Mount(*fMountPoint, fs, &fuse.MountConfig{})
	if err != nil {
		log.Fatalf("Mount: %v", err)
	}

	// Wait for it to be ready.
	if err = mfs.WaitForReady(context.Background()); err != nil {
		log.Fatalf("WaitForReady: %v", err)
	}

	// Wait for it to be unmounted.
	if err = mfs.Join(context.Background()); err != nil {
		log.Fatalf("Join: %v", err)
	}
}
