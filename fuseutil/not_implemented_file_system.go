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

func (fs *NotImplementedFileSystem) Init(
	ctx context.Context,
	req *fuse.InitRequest) (*fuse.InitResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) LookUpInode(
	ctx context.Context,
	req *fuse.LookUpInodeRequest) (*fuse.LookUpInodeResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) GetInodeAttributes(
	ctx context.Context,
	req *fuse.GetInodeAttributesRequest) (
	*fuse.GetInodeAttributesResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) SetInodeAttributes(
	ctx context.Context,
	req *fuse.SetInodeAttributesRequest) (
	*fuse.SetInodeAttributesResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ForgetInode(
	ctx context.Context,
	req *fuse.ForgetInodeRequest) (*fuse.ForgetInodeResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) MkDir(
	ctx context.Context,
	req *fuse.MkDirRequest) (*fuse.MkDirResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) CreateFile(
	ctx context.Context,
	req *fuse.CreateFileRequest) (*fuse.CreateFileResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) RmDir(
	ctx context.Context,
	req *fuse.RmDirRequest) (*fuse.RmDirResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) Unlink(
	ctx context.Context,
	req *fuse.UnlinkRequest) (*fuse.UnlinkResponse, error) {
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

func (fs *NotImplementedFileSystem) OpenFile(
	ctx context.Context,
	req *fuse.OpenFileRequest) (*fuse.OpenFileResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReadFile(
	ctx context.Context,
	req *fuse.ReadFileRequest) (*fuse.ReadFileResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) WriteFile(
	ctx context.Context,
	req *fuse.WriteFileRequest) (*fuse.WriteFileResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) FlushFile(
	ctx context.Context,
	req *fuse.FlushFileRequest) (*fuse.FlushFileResponse, error) {
	return nil, fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReleaseFileHandle(
	ctx context.Context,
	req *fuse.ReleaseFileHandleRequest) (*fuse.ReleaseFileHandleResponse, error) {
	return nil, fuse.ENOSYS
}
