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

// Package fuse enables writing and mounting user-space file systems.
//
// The primary elements of interest are:
//
//  *  The FileSystem interface, which defines the methods a file system must
//     implement.
//
//  *  fuseutil.NotImplementedFileSystem, which may be embedded to obtain
//     default implementations for all methods that are not of interest to a
//     particular file system.
//
//  *  Mount, a function that allows for mounting a file system.
//
// In order to use this package to mount file systems on OS X, the system must
// have FUSE for OS X installed: http://osxfuse.github.io/
package fuse
