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

	switch r.(type) {
	case *bazilfuse.InitRequest:
		to := &InitOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &LookUpInodeOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &GetInodeAttributesOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &SetInodeAttributesOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &ForgetInodeOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &MkDirOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &CreateFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &RmDirOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &UnlinkOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &OpenDirOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &ReadDirOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &ReleaseDirHandleOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &OpenFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &ReadFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &WriteFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &SyncFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &FlushFileOp{
		//TODO
		}
		o = to
		co = &to.commonOp

	case *bazilfuse.InitRequest:
		to := &ReleaseFileHandleOp{
		//TODO
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
