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

package fuse

import (
	"fmt"
	"unsafe"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/internal/buffer"
	"github.com/jacobsa/fuse/internal/fusekernel"
)

type internalOp struct {
}

////////////////////////////////////////////////////////////////////////
// Inodes
////////////////////////////////////////////////////////////////////////

type lookUpInodeOp struct {
	internalOp
	wrapped fuseops.LookUpInodeOp
}

func (o *lookUpInodeOp) kernelResponse(
	protocol fusekernel.Protocol) (b buffer.OutMessage) {
	size := fusekernel.EntryOutSize(protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.EntryOut)(b.Grow(size))
	convertChildInodeEntry(&o.wrapped.Entry, out)

	return
}

type getInodeAttributesOp struct {
	commonOp
	wrapped fuseops.GetInodeAttributesOp
}

func (o *GetInodeAttributesOp) DebugString() string {
	return fmt.Sprintf(
		"Inode: %d, Exp: %v, Attr: %s",
		o.Inode,
		o.AttributesExpiration,
		o.Attributes.DebugString())
}

func (o *GetInodeAttributesOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.AttrOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.AttrOut)(b.Grow(size))
	out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
	convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	return
}

type setInodeAttributesOp struct {
	commonOp
	wrapped fuseops.SetInodeAttributesOp
}

func (o *SetInodeAttributesOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.AttrOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.AttrOut)(b.Grow(size))
	out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
	convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	return
}

type forgetInodeOp struct {
	commonOp
	wrapped fuseops.ForgetInodeOp
}

func (o *ForgetInodeOp) kernelResponse() (b buffer.OutMessage) {
	// No response.
	return
}

////////////////////////////////////////////////////////////////////////
// Inode creation
////////////////////////////////////////////////////////////////////////

type mkDirOp struct {
	commonOp
	wrapped fuseops.MkDirOp
}

func (o *MkDirOp) ShortDesc() (desc string) {
	desc = fmt.Sprintf("MkDir(parent=%v, name=%q)", o.Parent, o.Name)
	return
}

func (o *MkDirOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.EntryOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.EntryOut)(b.Grow(size))
	convertChildInodeEntry(&o.Entry, out)

	return
}

type createFileOp struct {
	commonOp
	wrapped fuseops.CreateFileOp
}

func (o *CreateFileOp) ShortDesc() (desc string) {
	desc = fmt.Sprintf("CreateFile(parent=%v, name=%q)", o.Parent, o.Name)
	return
}

func (o *CreateFileOp) kernelResponse() (b buffer.OutMessage) {
	eSize := fusekernel.EntryOutSize(o.protocol)
	b = buffer.NewOutMessage(eSize + unsafe.Sizeof(fusekernel.OpenOut{}))

	e := (*fusekernel.EntryOut)(b.Grow(eSize))
	convertChildInodeEntry(&o.Entry, e)

	oo := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	oo.Fh = uint64(o.Handle)

	return
}

type createSymlinkOp struct {
	commonOp
	wrapped fuseops.CreateSymlinkOp
}

func (o *CreateSymlinkOp) ShortDesc() (desc string) {
	desc = fmt.Sprintf(
		"CreateSymlink(parent=%v, name=%q, target=%q)",
		o.Parent,
		o.Name,
		o.Target)

	return
}

func (o *CreateSymlinkOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.EntryOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.EntryOut)(b.Grow(size))
	convertChildInodeEntry(&o.Entry, out)

	return
}

////////////////////////////////////////////////////////////////////////
// Unlinking
////////////////////////////////////////////////////////////////////////

type renameOp struct {
	commonOp
	wrapped fuseops.RenameOp
}

func (o *RenameOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type rmDirOp struct {
	commonOp
	wrapped fuseops.RmDirOp
}

func (o *RmDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type unlinkOp struct {
	commonOp
	wrapped fuseops.UnlinkOp
}

func (o *UnlinkOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

////////////////////////////////////////////////////////////////////////
// Directory handles
////////////////////////////////////////////////////////////////////////

type openDirOp struct {
	commonOp
	wrapped fuseops.OpenDirOp
}

func (o *OpenDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
	out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	out.Fh = uint64(o.Handle)

	return
}

type readDirOp struct {
	commonOp
	wrapped fuseops.ReadDirOp
}

func (o *ReadDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Data)))
	b.Append(o.Data)
	return
}

type releaseDirHandleOp struct {
	commonOp
	wrapped fuseops.ReleaseDirHandleOp
}

func (o *ReleaseDirHandleOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

////////////////////////////////////////////////////////////////////////
// File handles
////////////////////////////////////////////////////////////////////////

type openFileOp struct {
	commonOp
	wrapped fuseops.OpenFileOp
}

func (o *OpenFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
	out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	out.Fh = uint64(o.Handle)

	return
}

type readFileOp struct {
	commonOp
	wrapped fuseops.ReadFileOp
}

func (o *ReadFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Data)))
	b.Append(o.Data)
	return
}

type writeFileOp struct {
	commonOp
	wrapped fuseops.WriteFileOp
}

func (o *WriteFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.WriteOut{}))
	out := (*fusekernel.WriteOut)(b.Grow(unsafe.Sizeof(fusekernel.WriteOut{})))
	out.Size = uint32(len(o.Data))

	return
}

type syncFileOp struct {
	commonOp
	wrapped fuseops.SyncFileOp
}

func (o *SyncFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type flushFileOp struct {
	commonOp
	wrapped fuseops.FlushFileOp
}

func (o *FlushFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type releaseFileHandleOp struct {
	commonOp
	wrapped fuseops.ReleaseFileHandleOp
}

func (o *ReleaseFileHandleOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

// A sentinel used for unknown ops. The user is expected to respond with a
// non-nil error.
type unknownOp struct {
	commonOp
	opCode uint32
	inode  InodeID
}

func (o *unknownOp) ShortDesc() (desc string) {
	desc = fmt.Sprintf("<opcode %d>(inode=%v)", o.opCode, o.inode)
	return
}

func (o *unknownOp) kernelResponse() (b buffer.OutMessage) {
	panic(fmt.Sprintf("Should never get here for unknown op: %s", o.ShortDesc()))
}

////////////////////////////////////////////////////////////////////////
// Reading symlinks
////////////////////////////////////////////////////////////////////////

type readSymlinkOp struct {
	commonOp
	wrapped fuseops.ReadSymlinkOp
}

func (o *ReadSymlinkOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Target)))
	b.AppendString(o.Target)
	return
}

////////////////////////////////////////////////////////////////////////
// Internal
////////////////////////////////////////////////////////////////////////

type internalStatFSOp struct {
	commonOp
}

func (o *InternalStatFSOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.StatfsOut{}))
	b.Grow(unsafe.Sizeof(fusekernel.StatfsOut{}))

	return
}

type internalInterruptOp struct {
	commonOp
	FuseID uint64
}

func (o *InternalInterruptOp) kernelResponse() (b buffer.OutMessage) {
	panic("Shouldn't get here.")
}

type internalInitOp struct {
	commonOp

	// In
	Kernel fusekernel.Protocol

	// Out
	Library      fusekernel.Protocol
	MaxReadahead uint32
	Flags        fusekernel.InitFlags
	MaxWrite     uint32
}

func (o *InternalInitOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.InitOut{}))
	out := (*fusekernel.InitOut)(b.Grow(unsafe.Sizeof(fusekernel.InitOut{})))

	out.Major = o.Library.Major
	out.Minor = o.Library.Minor
	out.MaxReadahead = o.MaxReadahead
	out.Flags = uint32(o.Flags)
	out.MaxWrite = o.MaxWrite

	return
}
