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
	"reflect"
	"sync"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/reqtrace"
	"golang.org/x/net/context"
)

// A helper for embedding common behavior.
type commonOp struct {
	opType      string
	r           bazilfuse.Request
	log         func(int, string, ...interface{})
	opsInFlight *sync.WaitGroup

	ctx    context.Context
	report reqtrace.ReportFunc
}

func describeOpType(t reflect.Type) (desc string) {
	// TODO(jacobsa): Make this nicer.
	desc = t.String()
	return
}

func (o *commonOp) init(
	ctx context.Context,
	opType reflect.Type,
	r bazilfuse.Request,
	log func(int, string, ...interface{}),
	opsInFlight *sync.WaitGroup) {
	// Initialize basic fields.
	o.opType = describeOpType(opType)
	o.r = r
	o.log = log
	o.opsInFlight = opsInFlight

	// Set up a trace span for this op.
	o.ctx, o.report = reqtrace.StartSpan(ctx, o.opType)
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

	o.report(err)

	o.Logf(
		"-> (%s) error: %v",
		o.opType,
		err)

	o.r.RespondError(err)
}

// Respond with the supplied response struct, which must be accepted by a
// method called Respond on o.r.
//
// Special case: nil means o.r.Respond accepts no parameters.
func (o *commonOp) respond(resp interface{}) {
	// We were successful.
	o.report(nil)

	// Find the Respond method.
	v := reflect.ValueOf(o.r)
	respond := v.MethodByName("Respond")

	// Special case: handle successful ops with no response struct.
	if resp == nil {
		o.Logf("-> (%s) OK", o.opType)
		respond.Call([]reflect.Value{})
		return
	}

	// Otherwise, pass along the response struct.
	o.Logf("-> %v", resp)
	respond.Call([]reflect.Value{reflect.ValueOf(resp)})
}
