// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package fuseutil

import (
	"github.com/jacobsa/fuse"
	"golang.org/x/net/context"
)

// Embed this within your file system type to inherit default implementations
// of all methods that return fuse.ENOSYS.
type NotImplementedFileSystem struct {
}

var _ fuse.FileSystem = &NotImplementedFileSystem{}

func (fs *NotImplementedFileSystem) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (*fuse.LookUpInodeResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ForgetInode(
	ctx context.Context,
	req *fuse.ForgetInodeRequest) (*fuse.ForgetInodeResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) OpenDir(
	ctx context.Context,
	req *fuse.OpenDirRequest) (*fuse.OpenDirResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReadDir(
	ctx context.Context,
	req *fuse.ReadDirRequest) (*fuse.ReadDirResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReleaseDirHandle(
	ctx context.Context,
	req *fuse.ReleaseDirHandleRequest) (*fuse.ReleaseDirHandleResponse, error) {
	return nil, fuse.ENOSYS
}
