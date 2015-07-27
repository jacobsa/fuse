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
	"syscall"
	"unsafe"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/internal/buffer"
	"github.com/jacobsa/fuse/internal/fusekernel"
)

// Given an op, return the response that should be sent to the kernel. If the
// op requires no response, return a nil response. If the op is unknown, return
// an error that should be sent to the kernel.
func kernelResponse(
	untyped fuseops.Op,
	protocol fusekernel.Protocol) (b buffer.OutMessage, err error) {
	switch o := untyped.(type) {
	case *fuseops.LookUpInodeOp:
		size := fusekernel.EntryOutSize(protocol)
		b = buffer.NewOutMessage(size)
		out := (*fusekernel.EntryOut)(b.Grow(size))
		convertChildInodeEntry(&o.Entry, out)

	case *fuseops.GetInodeAttributesOp:
		size := fusekernel.AttrOutSize(protocol)
		b = buffer.NewOutMessage(size)
		out := (*fusekernel.AttrOut)(b.Grow(size))
		out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
		convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	case *fuseops.SetInodeAttributesOp:
		size := fusekernel.AttrOutSize(protocol)
		b = buffer.NewOutMessage(size)
		out := (*fusekernel.AttrOut)(b.Grow(size))
		out.AttrValid, out.AttrValidNsec = convertExpirationTime(o.AttributesExpiration)
		convertAttributes(o.Inode, &o.Attributes, &out.Attr)

	case *fuseops.ForgetInodeOp:
		// No response.

	case *fuseops.MkDirOp:
		size := fusekernel.EntryOutSize(protocol)
		b = buffer.NewOutMessage(size)
		out := (*fusekernel.EntryOut)(b.Grow(size))
		convertChildInodeEntry(&o.Entry, out)

	case *fuseops.CreateFileOp:
		eSize := fusekernel.EntryOutSize(protocol)
		b = buffer.NewOutMessage(eSize + unsafe.Sizeof(fusekernel.OpenOut{}))

		e := (*fusekernel.EntryOut)(b.Grow(eSize))
		convertChildInodeEntry(&o.Entry, e)

		oo := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
		oo.Fh = uint64(o.Handle)

	case *fuseops.CreateSymlinkOp:
		size := fusekernel.EntryOutSize(protocol)
		b = buffer.NewOutMessage(size)
		out := (*fusekernel.EntryOut)(b.Grow(size))
		convertChildInodeEntry(&o.Entry, out)

	case *fuseops.RenameOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.RmDirOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.UnlinkOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.OpenDirOp:
		b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
		out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
		out.Fh = uint64(o.Handle)

	case *fuseops.ReadDirOp:
		b = buffer.NewOutMessage(uintptr(len(o.Data)))
		b.Append(o.Data)

	case *fuseops.ReleaseDirHandleOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.OpenFileOp:
		b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.OpenOut{}))
		out := (*fusekernel.OpenOut)(b.Grow(unsafe.Sizeof(fusekernel.OpenOut{})))
		out.Fh = uint64(o.Handle)

	case *fuseops.ReadFileOp:
		b = buffer.NewOutMessage(uintptr(len(o.Data)))
		b.Append(o.Data)

	case *fuseops.WriteFileOp:
		b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.WriteOut{}))
		out := (*fusekernel.WriteOut)(b.Grow(unsafe.Sizeof(fusekernel.WriteOut{})))
		out.Size = uint32(len(o.Data))

	case *fuseops.SyncFileOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.FlushFileOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.ReleaseFileHandleOp:
		b = buffer.NewOutMessage(0)

	case *fuseops.ReadSymlinkOp:
		b = buffer.NewOutMessage(uintptr(len(o.Target)))
		b.AppendString(o.Target)

	case *internalStatFSOp:
		b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.StatfsOut{}))
		b.Grow(unsafe.Sizeof(fusekernel.StatfsOut{}))

	case *internalInterruptOp:
		// No response.

	case *internalInitOp:
		b = buffer.NewOutMessage(unsafe.Sizeof(fusekernel.InitOut{}))
		out := (*fusekernel.InitOut)(b.Grow(unsafe.Sizeof(fusekernel.InitOut{})))

		out.Major = o.Library.Major
		out.Minor = o.Library.Minor
		out.MaxReadahead = o.MaxReadahead
		out.Flags = uint32(o.Flags)
		out.MaxWrite = o.MaxWrite

	default:
		err = syscall.ENOSYS
	}

	return
}

////////////////////////////////////////////////////////////////////////
// Internal
////////////////////////////////////////////////////////////////////////

// Common implementation for our "internal" ops that don't need to be pretty.
type internalOp struct {
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
