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

// Convert the supplied bazilfuse request struct to an Op.
//
// This function is an implementation detail of the fuse package, and must not
// be called by anyone else.
func Convert(r bazilfuse.Request) (o Op) {
	var co *commonOp

	switch r.(type) {
	case *bazilfuse.InitRequest:
		to := &InitOp{}
		o = to
		co = &to.commonOp

	default:
		panic("TODO")
	}

	co.init(r)
	return
}

// A helper for embedding common behavior.
type commonOp struct {
	ctx context.Context
	r   bazilfuse.Request
}

func (o *commonOp) init(r bazilfuse.Request)

func (o *commonOp) Header() OpHeader

func (o *commonOp) Context() context.Context {
	return o.ctx
}

func (o *commonOp) Respond(err error)
