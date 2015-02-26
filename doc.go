// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

// Package fuse enables writing and mounting user-space file systems.
//
// The primary elements of interest are:
//
//  *  The FileSystem interface, which defines the methods a file system must
//     implement.
//
//  *  NotImplementedFileSystem, which may be embedded to obtain default
//     implementations for all methods that are not of interest to a particular
//     file system.
//
//  *  Mount, a function that allows for mounting a file system.
//
// In order to use this package to mount file systems on OS X, the system must
// have FUSE for OS X installed: http://osxfuse.github.io/
package fuse
