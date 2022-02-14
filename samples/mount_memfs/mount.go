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
	"os/user"
	"strconv"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/memfs"
)

var fMountPoint = flag.String("mount_point", "", "Path to mount point.")

func main() {
	flag.Parse()

	if *fMountPoint == "" {
		log.Fatalf("You must set --mount_point.")
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		panic(err)
	}

	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		panic(err)
	}

	server := memfs.NewMemFS(uint32(uid), uint32(gid))

	cfg := &fuse.MountConfig{
		// Disable writeback caching so that pid is always available in OpContext
		DisableWritebackCaching: true,
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
