// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fuseops

import (
	"fmt"
)

type Filetype int

const (
	NoFiletype        Filetype = 0
	RegularFiletype            = 1
	DirectoryFiletype          = 2
	SymlinkFiletype            = 3
)

const (
	// Type values used in GCS metadata.
	NoFiletypeString        = "none"
	RegularFiletypeString   = "file"
	DirectoryFiletypeString = "directory"
	SymlinkFiletypeString   = "symlink"
)

func ParseFiletype(value string) (Filetype, error) {
	switch value {
	case NoFiletypeString:
		return NoFiletype, nil
	case RegularFiletypeString:
		return RegularFiletype, nil
	case DirectoryFiletypeString:
		return DirectoryFiletype, nil
	case SymlinkFiletypeString:
		return SymlinkFiletype, nil
	}
	return NoFiletype, fmt.Errorf("Failed to parse file type %s", value)
}

func (filetype Filetype) String() string {
	switch filetype {
	case RegularFiletype:
		return RegularFiletypeString
	case DirectoryFiletype:
		return DirectoryFiletypeString
	case SymlinkFiletype:
		return SymlinkFiletypeString
	}
	return NoFiletypeString
}
