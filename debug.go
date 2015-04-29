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
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

var fEnableDebug = flag.Bool(
	"fuse.debug",
	false,
	"Write FUSE debugging messages to stderr.")

var gLogger *log.Logger
var gLoggerOnce sync.Once

func initLogger() {
	if !flag.Parsed() {
		panic("initLogger called before flags available.")
	}

	var writer io.Writer = ioutil.Discard
	if *fEnableDebug {
		writer = os.Stderr
	}

	const flags = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
	gLogger = log.New(writer, "fuse: ", flags)
}

func getLogger() *log.Logger {
	gLoggerOnce.Do(initLogger)
	return gLogger
}
