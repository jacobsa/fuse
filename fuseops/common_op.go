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
	"flag"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/bazilfuse"
	"github.com/jacobsa/reqtrace"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

var fTraceByPID = flag.Bool(
	"fuse.trace_by_pid",
	false,
	"Enable a hacky mode that uses reqtrace to group all ops from each "+
		"individual PID. Not a good idea to use in production; races, bugs, and "+
		"resource leaks likely lurk.")

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
	name := t.String()

	// The usual case: a string that looks like "*fuseops.GetInodeAttributesOp".
	const prefix = "*fuseops."
	const suffix = "Op"
	if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
		desc = name[len(prefix) : len(name)-len(suffix)]
		return
	}

	// Otherwise, it's not clear what to do.
	desc = t.String()

	return
}

var gPIDMapMu sync.Mutex

// A map from PID to a traced context for that PID.
//
// GUARDED_BY(gPIDMapMu)
var gPIDMap = make(map[int]context.Context)

// Wait until the process completes, then close off the trace and remove the
// context from the map.
func reportWhenPIDGone(
	pid int,
	ctx context.Context,
	report reqtrace.ReportFunc) {
	// HACK(jacobsa): Poll for completion.
	const pollPeriod = 50 * time.Millisecond
	for {
		err := unix.Kill(pid, 0)
		if err == unix.ESRCH {
			break
		}

		if err != nil {
			panic(fmt.Errorf("Kill(%v): %v", pid, err))
		}

		time.Sleep(pollPeriod)
	}

	// Finish up.
	report(nil)

	gPIDMapMu.Lock()
	delete(gPIDMap, pid)
	gPIDMapMu.Unlock()
}

func (o *commonOp) maybeTraceByPID(
	in context.Context,
	pid int) (out context.Context) {
	// Is there anything to do?
	if !reqtrace.Enabled() || !*fTraceByPID {
		out = in
		return
	}

	gPIDMapMu.Lock()
	defer gPIDMapMu.Unlock()

	// Do we already have a traced context for this PID?
	if existing, ok := gPIDMap[pid]; ok {
		out = existing
		return
	}

	// Set up a new one and stick it in the map.
	var report reqtrace.ReportFunc
	out, report = reqtrace.Trace(in, fmt.Sprintf("PID %v", pid))
	gPIDMap[pid] = out

	// Ensure we close the trace and remove it from the map eventually.
	go reportWhenPIDGone(pid, out, report)

	return
}

func (o *commonOp) init(
	ctx context.Context,
	opType reflect.Type,
	r bazilfuse.Request,
	log func(int, string, ...interface{}),
	opsInFlight *sync.WaitGroup) {
	// Set up a context that reflects per-PID tracing if appropriate.
	ctx = o.maybeTraceByPID(ctx, int(r.Hdr().Pid))

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
