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

package fuseops

import (
	"log"
	"time"

	"github.com/jacobsa/fuse/internal/fusekernel"
	"github.com/jacobsa/fuse/internal/fuseshim"
	"golang.org/x/net/context"
)

// This function is an implementation detail of the fuse package, and must not
// be called by anyone else.
//
// Convert the supplied fuse kernel message to an Op. finished will be called
// with the error supplied to o.Respond when the user invokes that method,
// before a response is sent to the kernel. o.Respond will destroy the message.
//
// It is guaranteed that o != nil. If the op is unknown, a special unexported
// type will be used.
//
// The debug logging function and error logger may be nil.
func Convert(
	opCtx context.Context,
	m *fuseshim.Message,
	debugLogForOp func(int, string, ...interface{}),
	errorLogger *log.Logger,
	finished func(error)) (o Op) {
	var co *commonOp

	var io internalOp
	switch typed := r.(type) {
	case *fuseshim.LookupRequest:
		to := &LookUpInodeOp{
			bfReq:  typed,
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.GetattrRequest:
		to := &GetInodeAttributesOp{
			bfReq: typed,
			Inode: InodeID(typed.Header.Node),
		}
		io = to
		co = &to.commonOp

	case *fuseshim.SetattrRequest:
		to := &SetInodeAttributesOp{
			bfReq: typed,
			Inode: InodeID(typed.Header.Node),
		}

		if typed.Valid&fusekernel.SetattrSize != 0 {
			to.Size = &typed.Size
		}

		if typed.Valid&fusekernel.SetattrMode != 0 {
			to.Mode = &typed.Mode
		}

		if typed.Valid&fusekernel.SetattrAtime != 0 {
			to.Atime = &typed.Atime
		}

		if typed.Valid&fusekernel.SetattrMtime != 0 {
			to.Mtime = &typed.Mtime
		}

		io = to
		co = &to.commonOp

	case *fuseshim.ForgetRequest:
		to := &ForgetInodeOp{
			bfReq: typed,
			Inode: InodeID(typed.Header.Node),
			N:     typed.N,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.MkdirRequest:
		to := &MkDirOp{
			bfReq:  typed,
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.CreateRequest:
		to := &CreateFileOp{
			bfReq:  typed,
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.SymlinkRequest:
		to := &CreateSymlinkOp{
			bfReq:  typed,
			Parent: InodeID(typed.Header.Node),
			Name:   typed.NewName,
			Target: typed.Target,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.RenameRequest:
		to := &RenameOp{
			bfReq:     typed,
			OldParent: InodeID(typed.Header.Node),
			OldName:   typed.OldName,
			NewParent: InodeID(typed.NewDir),
			NewName:   typed.NewName,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.RemoveRequest:
		if typed.Dir {
			to := &RmDirOp{
				bfReq:  typed,
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}
			io = to
			co = &to.commonOp
		} else {
			to := &UnlinkOp{
				bfReq:  typed,
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}
			io = to
			co = &to.commonOp
		}

	case *fuseshim.OpenRequest:
		if typed.Dir {
			to := &OpenDirOp{
				bfReq: typed,
				Inode: InodeID(typed.Header.Node),
			}
			io = to
			co = &to.commonOp
		} else {
			to := &OpenFileOp{
				bfReq: typed,
				Inode: InodeID(typed.Header.Node),
			}
			io = to
			co = &to.commonOp
		}

	case *fuseshim.ReadRequest:
		if typed.Dir {
			to := &ReadDirOp{
				bfReq:  typed,
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: DirOffset(typed.Offset),
				Size:   typed.Size,
			}
			io = to
			co = &to.commonOp
		} else {
			to := &ReadFileOp{
				bfReq:  typed,
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: typed.Offset,
				Size:   typed.Size,
			}
			io = to
			co = &to.commonOp
		}

	case *fuseshim.ReleaseRequest:
		if typed.Dir {
			to := &ReleaseDirHandleOp{
				bfReq:  typed,
				Handle: HandleID(typed.Handle),
			}
			io = to
			co = &to.commonOp
		} else {
			to := &ReleaseFileHandleOp{
				bfReq:  typed,
				Handle: HandleID(typed.Handle),
			}
			io = to
			co = &to.commonOp
		}

	case *fuseshim.WriteRequest:
		to := &WriteFileOp{
			bfReq:  typed,
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
			Data:   typed.Data,
			Offset: typed.Offset,
		}
		io = to
		co = &to.commonOp

	case *fuseshim.FsyncRequest:
		// We don't currently support this for directories.
		if typed.Dir {
			to := &unknownOp{}
			io = to
			co = &to.commonOp
		} else {
			to := &SyncFileOp{
				bfReq:  typed,
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
			}
			io = to
			co = &to.commonOp
		}

	case *fuseshim.FlushRequest:
		to := &FlushFileOp{
			bfReq:  typed,
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
		}
		io = to
		co = &to.commonOp

	case *fuseshim.ReadlinkRequest:
		to := &ReadSymlinkOp{
			bfReq: typed,
			Inode: InodeID(typed.Header.Node),
		}
		io = to
		co = &to.commonOp

	default:
		to := &unknownOp{}
		io = to
		co = &to.commonOp
	}

	co.init(
		opCtx,
		io,
		r,
		debugLogForOp,
		errorLogger,
		finished)

	o = io
	return
}

func convertAttributes(
	inode InodeID,
	attr InodeAttributes,
	expiration time.Time) fuseshim.Attr {
	return fuseshim.Attr{
		Inode:  uint64(inode),
		Size:   attr.Size,
		Mode:   attr.Mode,
		Nlink:  uint32(attr.Nlink),
		Atime:  attr.Atime,
		Mtime:  attr.Mtime,
		Ctime:  attr.Ctime,
		Crtime: attr.Crtime,
		Uid:    attr.Uid,
		Gid:    attr.Gid,
		Valid:  convertExpirationTime(expiration),
	}
}

// Convert an absolute cache expiration time to a relative time from now for
// consumption by fuse.
func convertExpirationTime(t time.Time) (d time.Duration) {
	// Fuse represents durations as unsigned 64-bit counts of seconds and 32-bit
	// counts of nanoseconds (cf. http://goo.gl/EJupJV). The bazil.org/fuse
	// package converts time.Duration values to this form in a straightforward
	// way (cf. http://goo.gl/FJhV8j).
	//
	// So negative durations are right out. There is no need to cap the positive
	// magnitude, because 2^64 seconds is well longer than the 2^63 ns range of
	// time.Duration.
	d = t.Sub(time.Now())
	if d < 0 {
		d = 0
	}

	return
}

func convertChildInodeEntry(
	in *ChildInodeEntry,
	out *fuseshim.LookupResponse) {
	out.Node = fuseshim.NodeID(in.Child)
	out.Generation = uint64(in.Generation)
	out.Attr = convertAttributes(in.Child, in.Attributes, in.AttributesExpiration)
	out.EntryValid = convertExpirationTime(in.EntryExpiration)
}
