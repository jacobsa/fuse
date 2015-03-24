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
	"io"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
)

// An interface with a method for each op type in the fuseops package. This can
// be used in conjunction with NewFileSystemServer to avoid writing a "dispatch
// loop" that switches on op types, instead receiving typed method calls
// directly.
//
// Each method should fill in appropriate response fields for the supplied op
// and return an error status, but not call Repand.
//
// See NotImplementedFileSystem for a convenient way to embed default
// implementations for methods you don't care about.
type FileSystem interface {
	Init(*fuseops.InitOp) error
	LookUpInode(*fuseops.LookUpInodeOp) error
	GetInodeAttributes(*fuseops.GetInodeAttributesOp) error
	SetInodeAttributes(*fuseops.SetInodeAttributesOp) error
	ForgetInode(*fuseops.ForgetInodeOp) error
	MkDir(*fuseops.MkDirOp) error
	CreateFile(*fuseops.CreateFileOp) error
	RmDir(*fuseops.RmDirOp) error
	Unlink(*fuseops.UnlinkOp) error
	OpenDir(*fuseops.OpenDirOp) error
	ReadDir(*fuseops.ReadDirOp) error
	ReleaseDirHandle(*fuseops.ReleaseDirHandleOp) error
	OpenFile(*fuseops.OpenFileOp) error
	ReadFile(*fuseops.ReadFileOp) error
	WriteFile(*fuseops.WriteFileOp) error
	SyncFile(*fuseops.SyncFileOp) error
	FlushFile(*fuseops.FlushFileOp) error
	ReleaseFileHandle(*fuseops.ReleaseFileHandleOp) error
}

// Create a fuse.Server that serves ops by calling the associated FileSystem
// method and then calling Op.Respond with the resulting error. Unsupported ops
// are responded to directly with ENOSYS.
func NewFileSystemServer(fs FileSystem) fuse.Server {
	return fileSystemServer{fs}
}

type fileSystemServer struct {
	fs FileSystem
}

func (s fileSystemServer) ServeOps(c *fuse.Connection) {
	for {
		op, err := c.ReadOp()
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		switch typed := op.(type) {
		default:
			op.Respond(fuse.ENOSYS)

		case *fuseops.InitOp:
			op.Respond(s.fs.Init(typed))

		case *fuseops.LookUpInodeOp:
			op.Respond(s.fs.LookUpInode(typed))

		case *fuseops.GetInodeAttributesOp:
			op.Respond(s.fs.GetInodeAttributes(typed))

		case *fuseops.SetInodeAttributesOp:
			op.Respond(s.fs.SetInodeAttributes(typed))

		case *fuseops.ForgetInodeOp:
			op.Respond(s.fs.ForgetInode(typed))

		case *fuseops.MkDirOp:
			op.Respond(s.fs.MkDir(typed))

		case *fuseops.CreateFileOp:
			op.Respond(s.fs.CreateFile(typed))

		case *fuseops.RmDirOp:
			op.Respond(s.fs.RmDir(typed))

		case *fuseops.UnlinkOp:
			op.Respond(s.fs.Unlink(typed))

		case *fuseops.OpenDirOp:
			op.Respond(s.fs.OpenDir(typed))

		case *fuseops.ReadDirOp:
			op.Respond(s.fs.ReadDir(typed))

		case *fuseops.ReleaseDirHandleOp:
			op.Respond(s.fs.ReleaseDirHandle(typed))

		case *fuseops.OpenFileOp:
			op.Respond(s.fs.OpenFile(typed))

		case *fuseops.ReadFileOp:
			op.Respond(s.fs.ReadFile(typed))

		case *fuseops.WriteFileOp:
			op.Respond(s.fs.WriteFile(typed))

		case *fuseops.SyncFileOp:
			op.Respond(s.fs.SyncFile(typed))

		case *fuseops.FlushFileOp:
			op.Respond(s.fs.FlushFile(typed))

		case *fuseops.ReleaseFileHandleOp:
			op.Respond(s.fs.ReleaseFileHandle(typed))
		}
	}
}
