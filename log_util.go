// Copyright 2026 Google Inc. All Rights Reserved.
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
	"log"
)

// firstLogger returns the first non-nil logger from the provided list.
// If all are nil, it returns no-op logger
func FirstLogger(loggers ...*log.Logger) *log.Logger {
	for _, l := range loggers {
		if l != nil {
			return l
		}
	}
	return nil
}
