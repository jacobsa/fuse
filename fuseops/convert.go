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
	"bytes"
	"log"
	"time"
	"unsafe"

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
	protocol fusekernel.Protocol,
	debugLogForOp func(int, string, ...interface{}),
	errorLogger *log.Logger,
	finished func(error)) (o Op) {
	var co *commonOp

	var io internalOp
	switch m.Hdr.Opcode {
	case fusekernel.OpLookup:
		buf := m.Bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}

		to := &LookUpInodeOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(buf[:n-1]),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpGetattr:
		to := &GetInodeAttributesOp{
			Inode: InodeID(m.Header().Node),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpSetattr:
		in := (*fusekernel.SetattrIn)(m.Data())
		if m.Len() < unsafe.Sizeof(*in) {
			goto corrupt
		}

		to := &SetInodeAttributesOp{
			Inode: InodeID(m.Header().Node),
		}

		valid := fusekernel.SetattrValid(in.Valid)
		if valid&fusekernel.SetattrSize != 0 {
			to.Size = &in.Size
		}

		if valid&fusekernel.SetattrMode != 0 {
			mode := fuseshim.FileMode(in.Mode)
			to.Mode = &mode
		}

		if valid&fusekernel.SetattrAtime != 0 {
			t := time.Unix(int64(in.Atime), int64(in.AtimeNsec))
			to.Atime = &t
		}

		if valid&fusekernel.SetattrMtime != 0 {
			t := time.Unix(int64(in.Mtime), int64(in.MtimeNsec))
			to.Mtime = &t
		}

		io = to
		co = &to.commonOp

	case fusekernel.OpForget:
		in := (*fusekernel.ForgetIn)(m.Data())
		if m.Len() < unsafe.Sizeof(*in) {
			goto corrupt
		}

		to := &ForgetInodeOp{
			Inode: InodeID(m.Header().Node),
			N:     in.Nlookup,
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpMkdir:
		size := fusekernel.MkdirInSize(protocol)
		if m.Len() < size {
			goto corrupt
		}
		in := (*fusekernel.MkdirIn)(m.Data())
		name := m.Bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		name = name[:i]

		to := &MkDirOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(name),
			Mode:   fuseshim.FileMode(in.Mode),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpCreate:
		size := fusekernel.CreateInSize(protocol)
		if m.Len() < size {
			goto corrupt
		}
		in := (*fusekernel.CreateIn)(m.Data())
		name := m.Bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		name = name[:i]

		to := &CreateFileOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(name),
			Mode:   fuseshim.FileMode(in.Mode),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpSymlink:
		// m.Bytes() is "newName\0target\0"
		names := m.Bytes()
		if len(names) == 0 || names[len(names)-1] != 0 {
			goto corrupt
		}
		i := bytes.IndexByte(names, '\x00')
		if i < 0 {
			goto corrupt
		}
		newName, target := names[0:i], names[i+1:len(names)-1]

		to := &CreateSymlinkOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(newName),
			Target: string(target),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpRename:
		in := (*fusekernel.RenameIn)(m.Data())
		if m.Len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		names := m.Bytes()[unsafe.Sizeof(*in):]
		// names should be "old\x00new\x00"
		if len(names) < 4 {
			goto corrupt
		}
		if names[len(names)-1] != '\x00' {
			goto corrupt
		}
		i := bytes.IndexByte(names, '\x00')
		if i < 0 {
			goto corrupt
		}
		oldName, newName := names[:i], names[i+1:len(names)-1]

		to := &RenameOp{
			OldParent: InodeID(m.Header().Node),
			OldName:   string(oldName),
			NewParent: InodeID(in.Newdir),
			NewName:   string(newName),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpUnlink:
		buf := m.Bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}

		to := &UnlinkOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(buf[:n-1]),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpRmdir:
		buf := m.Bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}

		to := &RmDirOp{
			Parent: InodeID(m.Header().Node),
			Name:   string(buf[:n-1]),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpOpen:
		to := &OpenFileOp{
			Inode: InodeID(m.Header().Node),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpOpendir:
		to := &OpenDirOp{
			Inode: InodeID(m.Header().Node),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpRead:
		in := (*fusekernel.ReadIn)(m.Data())
		if m.Len() < fusekernel.ReadInSize(protocol) {
			goto corrupt
		}

		to := &ReadFileOp{
			Inode:  InodeID(m.Header().Node),
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Size:   int(in.Size),
		}
		io = to
		co = &to.commonOp

	case fusekernel.OpReaddir:
		in := (*fusekernel.ReadIn)(m.Data())
		if m.Len() < fusekernel.ReadInSize(protocol) {
			goto corrupt
		}

		to := &ReadDirOp{
			Inode:  InodeID(m.Header().Node),
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Size:   int(in.Size),
		}
		io = to
		co = &to.commonOp

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
