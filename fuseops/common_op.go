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
	"sync"

	"github.com/jacobsa/bazilfuse"
	"golang.org/x/net/context"
)

// A helper for embedding common behavior.
type commonOp struct {
	opType      string
	ctx         context.Context
	r           bazilfuse.Request
	log         func(int, string, ...interface{})
	opsInFlight *sync.WaitGroup
}

func (o *commonOp) init(
	opType string,
	r bazilfuse.Request,
	log func(int, string, ...interface{}),
	opsInFlight *sync.WaitGroup) {
	o.opType = opType
	o.ctx = context.Background()
	o.r = r
	o.log = log
	o.opsInFlight = opsInFlight
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

func (o *commonOp) Logf(format string, v ...interface{}) {
	const calldepth = 2
	o.log(calldepth, format, v...)
}

func (o *commonOp) respondErr(err error) {
	if err == nil {
		panic("Expect non-nil here.")
	}

	o.Logf(
		"-> (%s) error: %v",
		o.opType,
		err)

	o.r.RespondError(err)
}
