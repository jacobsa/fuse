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
	"github.com/jacobsa/fuse/fuseops"
)

// A FileSystem that returns fuse.ENOSYS for all methods. Embed this in your
// struct to inherit default implementations for the methods you don't care
// about, ensuring your struct will continue to implement FileSystem even as
// new methods are added.
type NotImplementedFileSystem struct {
}

var _ FileSystem = &NotImplementedFileSystem{}

func (fs *NotImplementedFileSystem) Init(
	op *fuseops.InitOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) LookUpInode(
	op *fuseops.LookUpInodeOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) GetInodeAttributes(
	op *fuseops.GetInodeAttributesOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) SetInodeAttributes(
	op *fuseops.SetInodeAttributesOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ForgetInode(
	op *fuseops.ForgetInodeOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) MkDir(
	op *fuseops.MkDirOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) CreateFile(
	op *fuseops.CreateFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) RmDir(
	op *fuseops.RmDirOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) Unlink(
	op *fuseops.UnlinkOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) OpenDir(
	op *fuseops.OpenDirOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReadDir(
	op *fuseops.ReadDirOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReleaseDirHandle(
	op *fuseops.ReleaseDirHandleOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) OpenFile(
	op *fuseops.OpenFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReadFile(
	op *fuseops.ReadFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) WriteFile(
	op *fuseops.WriteFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) SyncFile(
	op *fuseops.SyncFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) FlushFile(
	op *fuseops.FlushFileOp) error {
	return fuse.ENOSYS
}

func (fs *NotImplementedFileSystem) ReleaseFileHandle(
	op *fuseops.ReleaseFileHandleOp) error {
	return fuse.ENOSYS
}
