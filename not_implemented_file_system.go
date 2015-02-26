// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package fuse

import "golang.org/x/net/context"

// Embed this within your file system type to inherit default implementations
// of all methods that return ENOSYS.
type NotImplementedFileSystem struct {
}

var _ FileSystem = &NotImplementedFileSystem{}

func (fs *NotImplementedFileSystem) LookUpInode(
	ctx context.Context,
	req *LookUpInodeRequest) (*LookUpInodeResponse, error) {
	return nil, ENOSYS
}

func (fs *NotImplementedFileSystem) ForgetInode(
	ctx context.Context,
	req *ForgetInodeRequest) (*ForgetInodeResponse, error) {
	return nil, ENOSYS
}

func (fs *NotImplementedFileSystem) OpenDir(
	ctx context.Context,
	req *OpenDirRequest) (*OpenDirResponse, error) {
	return nil, ENOSYS
}

func (fs *NotImplementedFileSystem) ReleaseHandle(
	ctx context.Context,
	req *ReleaseHandleRequest) (*ReleaseHandleResponse, error) {
	return nil, ENOSYS
}
