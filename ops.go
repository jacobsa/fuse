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

type opCommon struct {
}

////////////////////////////////////////////////////////////////////////
// Inodes
////////////////////////////////////////////////////////////////////////

type lookUpInodeOp struct {
	opCommon
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
	opCommon
	wrapped fuseops.GetInodeAttributesOp
}

func (o *getInodeAttributesOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.AttrOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.AttrOut)(b.Grow(size))
	out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
	convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	return
}

type setInodeAttributesOp struct {
	opCommon
	wrapped fuseops.SetInodeAttributesOp
}

func (o *setInodeAttributesOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.AttrOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.AttrOut)(b.Grow(size))
	out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
	convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	return
}

type forgetInodeOp struct {
	opCommon
	wrapped fuseops.ForgetInodeOp
}

func (o *forgetInodeOp) kernelResponse() (b buffer.OutMessage) {
	// No response.
	return
}

////////////////////////////////////////////////////////////////////////
// Inode creation
////////////////////////////////////////////////////////////////////////

type mkDirOp struct {
	opCommon
	wrapped fuseops.MkDirOp
}

func (o *mkDirOp) kernelResponse() (b buffer.OutMessage) {
	size := fusekernel.EntryOutSize(o.protocol)
	b = buffer.NewOutMessage(size)
	out := (*fusekernel.EntryOut)(b.Grow(size))
	convertChildInodeEntry(&o.Entry, out)

	return
}

type createFileOp struct {
	opCommon
	wrapped fuseops.CreateFileOp
}

func (o *createFileOp) kernelResponse() (b buffer.OutMessage) {
	eSize := fusekernel.EntryOutSize(o.protocol)
	b = buffer.NewOutMessage(eSize + unsafe.Sizeof(fusekernel.OpenOut{}))

	e := (*fusekernel.EntryOut)(b.Grow(eSize))
	convertChildInodeEntry(&o.Entry, e)

	oo := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	oo.Fh = uint64(o.Handle)

	return
}

type createSymlinkOp struct {
	opCommon
	wrapped fuseops.CreateSymlinkOp
}

func (o *createSymlinkOp) kernelResponse() (b buffer.OutMessage) {
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
	opCommon
	wrapped fuseops.RenameOp
}

func (o *renameOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type rmDirOp struct {
	opCommon
	wrapped fuseops.RmDirOp
}

func (o *rmDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type unlinkOp struct {
	opCommon
	wrapped fuseops.UnlinkOp
}

func (o *unlinkOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

////////////////////////////////////////////////////////////////////////
// Directory handles
////////////////////////////////////////////////////////////////////////

type openDirOp struct {
	opCommon
	wrapped fuseops.OpenDirOp
}

func (o *openDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
	out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	out.Fh = uint64(o.Handle)

	return
}

type readDirOp struct {
	opCommon
	wrapped fuseops.ReadDirOp
}

func (o *readDirOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Data)))
	b.Append(o.Data)
	return
}

type releaseDirHandleOp struct {
	opCommon
	wrapped fuseops.ReleaseDirHandleOp
}

func (o *releaseDirHandleOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

////////////////////////////////////////////////////////////////////////
// File handles
////////////////////////////////////////////////////////////////////////

type openFileOp struct {
	opCommon
	wrapped fuseops.OpenFileOp
}

func (o *openFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
	out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
	out.Fh = uint64(o.Handle)

	return
}

type readFileOp struct {
	opCommon
	wrapped fuseops.ReadFileOp
}

func (o *readFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Data)))
	b.Append(o.Data)
	return
}

type writeFileOp struct {
	opCommon
	wrapped fuseops.WriteFileOp
}

func (o *writeFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.WriteOut{}))
	out := (*fusekernel.WriteOut)(b.Grow(unsafe.Sizeof(fusekernel.WriteOut{})))
	out.Size = uint32(len(o.Data))

	return
}

type syncFileOp struct {
	opCommon
	wrapped fuseops.SyncFileOp
}

func (o *syncFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type flushFileOp struct {
	opCommon
	wrapped fuseops.FlushFileOp
}

func (o *flushFileOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

type releaseFileHandleOp struct {
	opCommon
	wrapped fuseops.ReleaseFileHandleOp
}

func (o *releaseFileHandleOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(0)
	return
}

// A sentinel used for unknown ops. The user is expected to respond with a
// non-nil error.
type unknownOp struct {
	opCommon
	opCode uint32
	inode  fuseops.InodeID
}

func (o *unknownOp) kernelResponse() (b buffer.OutMessage) {
	panic(fmt.Sprintf("Should never get here for unknown op: %s", o.ShortDesc()))
}

////////////////////////////////////////////////////////////////////////
// Reading symlinks
////////////////////////////////////////////////////////////////////////

type readSymlinkOp struct {
	opCommon
	wrapped fuseops.ReadSymlinkOp
}

func (o *readSymlinkOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(uintptr(len(o.Target)))
	b.AppendString(o.Target)
	return
}

////////////////////////////////////////////////////////////////////////
// Internal
////////////////////////////////////////////////////////////////////////

// Common implementation for our "internal" ops that don't need to be pretty.
type internalOp struct {
	opCommon
}

func (o *internalOp) ShortDesc() string   { return "<internalOp>" }
func (o *internalOp) DebugString() string { return "<internalOp>" }

type internalStatFSOp struct {
	internalOp
}

func (o *internalStatFSOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.StatfsOut{}))
	b.Grow(unsafe.Sizeof(fusekernel.StatfsOut{}))

	return
}

type internalInterruptOp struct {
	internalOp
	FuseID uint64
}

func (o *internalInterruptOp) kernelResponse() (b buffer.OutMessage) {
	panic("Shouldn't get here.")
}

type internalInitOp struct {
	internalOp

	// In
	Kernel fusekernel.Protocol

	// Out
	Library      fusekernel.Protocol
	MaxReadahead uint32
	Flags        fusekernel.InitFlags
	MaxWrite     uint32
}

func (o *internalInitOp) kernelResponse() (b buffer.OutMessage) {
	b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.InitOut{}))
	out := (*fusekernel.InitOut)(b.Grow(unsafe.Sizeof(fusekernel.InitOut{})))

	out.Major = o.Library.Major
	out.Minor = o.Library.Minor
	out.MaxReadahead = o.MaxReadahead
	out.Flags = uint32(o.Flags)
	out.MaxWrite = o.MaxWrite

	return
}
