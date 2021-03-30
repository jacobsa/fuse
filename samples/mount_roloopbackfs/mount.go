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

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/roloopbackfs"
)

var fPhysicalPath = flag.String("path", "", "Physical path to loopback.")
var fMountPoint = flag.String("mount_point", "", "Path to mount point.")

var fDebug = flag.Bool("debug", false, "Enable debug logging.")

func main() {
	flag.Parse()

	debugLogger := log.New(os.Stdout, "fuse: ", 0)
	errorLogger := log.New(os.Stderr, "fuse: ", 0)

	if *fPhysicalPath == "" {
		log.Fatalf("You must set --path.")
	}

	if *fMountPoint == "" {
		log.Fatalf("You must set --mount_point.")
	}

	err := os.MkdirAll(*fMountPoint, 0777)
	if err != nil {
		log.Fatalf("Failed to create mount point at '%v'", *fMountPoint)
	}

	server, err := roloopbackfs.NewReadonlyLoopbackServer(*fPhysicalPath, errorLogger)
	if err != nil {
		log.Fatalf("makeFS: %v", err)
	}

	cfg := &fuse.MountConfig{
		ReadOnly:    true,
		ErrorLogger: errorLogger,
	}

	if *fDebug {
		cfg.DebugLogger = debugLogger
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
