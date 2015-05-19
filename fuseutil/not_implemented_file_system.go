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

// A FileSystem that responds to all ops with fuse.ENOSYS. Embed this in your
// struct to inherit default implementations for the methods you don't care
// about, ensuring your struct will continue to implement FileSystem even as
// new methods are added.
type NotImplementedFileSystem struct {
}

var _ FileSystem = &NotImplementedFileSystem{}

func (fs *NotImplementedFileSystem) Init(
	op *fuseops.InitOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) LookUpInode(
	op *fuseops.LookUpInodeOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) GetInodeAttributes(
	op *fuseops.GetInodeAttributesOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) SetInodeAttributes(
	op *fuseops.SetInodeAttributesOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ForgetInode(
	op *fuseops.ForgetInodeOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) MkDir(
	op *fuseops.MkDirOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) CreateFile(
	op *fuseops.CreateFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) CreateSymlink(
	op *fuseops.CreateSymlinkOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) RmDir(
	op *fuseops.RmDirOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) Unlink(
	op *fuseops.UnlinkOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) OpenDir(
	op *fuseops.OpenDirOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ReadDir(
	op *fuseops.ReadDirOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ReleaseDirHandle(
	op *fuseops.ReleaseDirHandleOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) OpenFile(
	op *fuseops.OpenFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ReadFile(
	op *fuseops.ReadFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) WriteFile(
	op *fuseops.WriteFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) SyncFile(
	op *fuseops.SyncFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) FlushFile(
	op *fuseops.FlushFileOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ReleaseFileHandle(
	op *fuseops.ReleaseFileHandleOp) {
	op.Respond(fuse.ENOSYS)
}

func (fs *NotImplementedFileSystem) ReadSymlink(
	op *fuseops.ReadSymlinkOp) {
	op.Respond(fuse.ENOSYS)
}
