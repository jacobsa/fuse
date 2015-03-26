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

// Package fuseops contains implementations of the fuse.Op interface that may
// be returned by fuse.Connection.ReadOp. See documentation in that package for
// more.
package fuseops

import (
	"log"
	"reflect"
	"time"

	"github.com/jacobsa/bazilfuse"
	"golang.org/x/net/context"
)

// Convert the supplied bazilfuse request struct to an Op, returning nil if it
// is unknown.
//
// This function is an implementation detail of the fuse package, and must not
// be called by anyone else.
func Convert(r bazilfuse.Request, logger *log.Logger) (o Op) {
	var co *commonOp

	switch typed := r.(type) {
	case *bazilfuse.InitRequest:
		to := &InitOp{
			maxReadahead: typed.MaxReadahead,
		}

		o = to
		co = &to.commonOp

	case *bazilfuse.LookupRequest:
		to := &LookUpInodeOp{
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.GetattrRequest:
		to := &GetInodeAttributesOp{
			Inode: InodeID(typed.Header.Node),
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.SetattrRequest:
		to := &SetInodeAttributesOp{
			Inode: InodeID(typed.Header.Node),
		}

		if typed.Valid&bazilfuse.SetattrSize != 0 {
			to.Size = &typed.Size
		}

		if typed.Valid&bazilfuse.SetattrMode != 0 {
			to.Mode = &typed.Mode
		}

		if typed.Valid&bazilfuse.SetattrAtime != 0 {
			to.Atime = &typed.Atime
		}

		if typed.Valid&bazilfuse.SetattrMtime != 0 {
			to.Mtime = &typed.Mtime
		}

		o = to
		co = &to.commonOp

	case *bazilfuse.MkdirRequest:
		to := &MkDirOp{
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.CreateRequest:
		to := &CreateFileOp{
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
			Flags:  typed.Flags,
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.RemoveRequest:
		if typed.Dir {
			to := &RmDirOp{
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}
			o = to
			co = &to.commonOp
		} else {
			to := &UnlinkOp{
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}
			o = to
			co = &to.commonOp
		}

	case *bazilfuse.OpenRequest:
		if typed.Dir {
			to := &OpenDirOp{
				Inode: InodeID(typed.Header.Node),
				Flags: typed.Flags,
			}
			o = to
			co = &to.commonOp
		} else {
			to := &OpenFileOp{
				Inode: InodeID(typed.Header.Node),
				Flags: typed.Flags,
			}
			o = to
			co = &to.commonOp
		}

	case *bazilfuse.ReadRequest:
		if typed.Dir {
			to := &ReadDirOp{
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: DirOffset(typed.Offset),
				Size:   typed.Size,
			}
			o = to
			co = &to.commonOp
		} else {
			to := &ReadFileOp{
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: typed.Offset,
				Size:   typed.Size,
			}
			o = to
			co = &to.commonOp
		}

	case *bazilfuse.ReleaseRequest:
		if typed.Dir {
			to := &ReleaseDirHandleOp{
				Handle: HandleID(typed.Handle),
			}
			o = to
			co = &to.commonOp
		} else {
			to := &ReleaseFileHandleOp{
				Handle: HandleID(typed.Handle),
			}
			o = to
			co = &to.commonOp
		}

	case *bazilfuse.WriteRequest:
		to := &WriteFileOp{
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
			Data:   typed.Data,
			Offset: typed.Offset,
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.FsyncRequest:
		// We don't currently support this for directories.
		if typed.Dir {
			return
		}

		to := &SyncFileOp{
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.FlushRequest:
		to := &FlushFileOp{
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
		}
		o = to
		co = &to.commonOp

	default:
		return
	}

	co.init(reflect.TypeOf(o).String(), r, logger)
	return
}

func convertAttributes(inode InodeID, attr InodeAttributes) bazilfuse.Attr {
	return bazilfuse.Attr{
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
	out *bazilfuse.LookupResponse) {
	out.Node = bazilfuse.NodeID(in.Child)
	out.Generation = uint64(in.Generation)
	out.Attr = convertAttributes(in.Child, in.Attributes)
	out.AttrValid = convertExpirationTime(in.AttributesExpiration)
	out.EntryValid = convertExpirationTime(in.EntryExpiration)
}

// A helper for embedding common behavior.
type commonOp struct {
	opType string
	ctx    context.Context
	r      bazilfuse.Request
	logger *log.Logger
}

func (o *commonOp) init(
	opType string,
	r bazilfuse.Request,
	logger *log.Logger) {
	o.opType = opType
	o.ctx = context.Background()
	o.r = r
	o.logger = logger
}

func (o *commonOp) Header() OpHeader {
	bh := o.r.Hdr()
	return OpHeader{
		Uid: bh.Uid,
		Gid: bh.Gid,
	}
}

func (o *commonOp) Context() context.Context {
	return o.ctx
}

func (o *commonOp) respondErr(err error) {
	if err == nil {
		panic("Expect non-nil here.")
	}

	o.logger.Printf(
		"Responding with error to %s: %v",
		o.opType,
		err)

	o.r.RespondError(err)
}
