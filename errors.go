// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package fuse

import (
	"syscall"

	bazilfuse "bazil.org/fuse"
)

const (
	// Errors corresponding to kernel error numbers. These may be treated
	// specially when returned by a FileSystem method.
	EIO       = bazilfuse.EIO
	ENOENT    = bazilfuse.ENOENT
	ENOSYS    = bazilfuse.ENOSYS
	ENOTEMPTY = bazilfuse.Errno(syscall.ENOTEMPTY)
)
