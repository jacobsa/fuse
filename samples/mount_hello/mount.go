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
	"context"
	"flag"
	"log"
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/hellofs"
	"github.com/jacobsa/timeutil"
)

var fType = flag.String("type", "", "The name of the samples/ sub-dir.")
var fMountPoint = flag.String("mount_point", "", "Path to mount point.")
var fReadyFile = flag.Uint64("ready_file", 0, "FD to signal when ready.")

var fReadOnly = flag.Bool("read_only", false, "Mount in read-only mode.")
var fDebug = flag.Bool("debug", false, "Enable debug logging.")

func main() {
	flag.Parse()

	// Create an appropriate file system.
	server, err := hellofs.NewHelloFS(timeutil.RealClock())
	if err != nil {
		log.Fatalf("makeFS: %v", err)
	}

	// Mount the file system.
	if *fMountPoint == "" {
		log.Fatalf("You must set --mount_point.")
	}

	cfg := &fuse.MountConfig{
		ReadOnly: *fReadOnly,
		FuseImpl: fuse.FUSEImplMacFUSE,
	}

	if *fDebug {
		cfg.DebugLogger = log.New(os.Stderr, "fuse: ", 0)
	}

	mfs, err := fuse.Mount(*fMountPoint, server, cfg)
	if err != nil {
		log.Fatalf("Mount: %v", err)
	}

	// Wait for it to be unmounted.
	if err = mfs.Join(context.Background()); err != nil {
		log.Fatalf("Join: %v", err)
	}
}
