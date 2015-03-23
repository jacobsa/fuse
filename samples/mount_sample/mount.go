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
	"fmt"
	"log"
	"os"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/flushfs"
	"golang.org/x/net/context"
)

var fType = flag.String("type", "", "The name of the samples/ sub-dir.")
var fMountPoint = flag.String("mount_point", "", "Path to mount point.")

var fFlushesFile = flag.Uint64("flushfs.flushes_file", 0, "")
var fFsyncsFile = flag.Uint64("flushfs.fsyncs_file", 0, "")
var fFlushError = flag.Int("flushfs.flush_error", 0, "")
var fFsyncError = flag.Int("flushfs.fsync_error", 0, "")

func makeFlushFS() (fs fuse.FileSystem, err error) {
	// Check the flags.
	if *fFlushesFile == 0 || *fFsyncsFile == 0 {
		err = fmt.Errorf("You must set the flushfs flags.")
		return
	}

	// Set up the files.
	flushes := os.NewFile(uintptr(*fFlushesFile), "(flushes file)")
	fsyncs := os.NewFile(uintptr(*fFsyncsFile), "(fsyncs file)")

	// Set up errors.
	var flushErr error
	var fsyncErr error

	if *fFlushError != 0 {
		flushErr = bazilfuse.Errno(*fFlushError)
	}

	if *fFsyncError != 0 {
		fsyncErr = bazilfuse.Errno(*fFsyncError)
	}

	// Report flushes and fsyncs by writing the contents followed by a newline.
	report := func(f *os.File, outErr error) func(string) error {
		return func(s string) (err error) {
			buf := []byte(s)
			buf = append(buf, '\n')

			_, err = f.Write(buf)
			if err != nil {
				err = fmt.Errorf("Write: %v", err)
				return
			}

			err = outErr
			return
		}
	}

	reportFlush := report(flushes, flushErr)
	reportFsync := report(fsyncs, fsyncErr)

	// Create the file system.
	fs, err = flushfs.NewFileSystem(reportFlush, reportFsync)

	return
}

func makeFS() (fs fuse.FileSystem, err error) {
	switch *fType {
	default:
		err = fmt.Errorf("Unknown FS type: %v", *fType)

	case "flushfs":
		fs, err = makeFlushFS()
	}

	return
}

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
