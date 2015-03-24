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
	"github.com/jacobsa/bazilfuse"
	"golang.org/x/net/context"
)

// Convert the supplied bazilfuse request struct to an Op, returning nil if it
// is unknown.
//
// This function is an implementation detail of the fuse package, and must not
// be called by anyone else.
func Convert(r bazilfuse.Request) (o Op) {
	var co *commonOp

	switch typed := r.(type) {
	case *bazilfuse.InitRequest:
		to := &InitOp{}
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
		//TODO

	case *bazilfuse.ReadRequest:
		//TODO

	case *bazilfuse.ReleaseRequest:
		//TODO

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

	co.init(r)
	return
}

// A helper for embedding common behavior.
type commonOp struct {
	ctx context.Context
	r   bazilfuse.Request
}

func (o *commonOp) init(r bazilfuse.Request) {
	o.ctx = context.Background()
	o.r = r
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
	if err != nil {
		panic("Expect non-nil here.")
	}

	o.r.RespondError(err)
}
