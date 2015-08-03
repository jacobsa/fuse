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

package errorfs

import (
	"errors"
	"reflect"
	"syscall"

	"github.com/jacobsa/fuse/fuseutil"
)

const FooContents = "xxxx"

// A file system whose sole contents are a file named "foo" containing the
// string defined by FooContents.
//
// The file system can be configured to returned canned errors for particular
// operations using the method SetError.
type FS interface {
	fuseutil.FileSystem

	// Cause the file system to return the supplied error for all future
	// operations matching the supplied type.
	SetError(t reflect.Type, err syscall.Errno)
}

func New() (fs FS, err error) {
	err = errors.New("TODO")
	return
}
