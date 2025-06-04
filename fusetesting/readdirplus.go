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

package fusetesting

import (
	"os"
	"sort"
)

type sortedEntriesPlus []os.FileInfo

func (f sortedEntriesPlus) Len() int           { return len(f) }
func (f sortedEntriesPlus) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f sortedEntriesPlus) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// Read the directory with the given name and return a list of directory
// entries with their attributes, sorted by name.
func ReadDirPlusPicky(dirname string) (entriesPlus []os.FileInfo, err error) {
	// Read directory contents.
	dirEntries, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}

	// Get FileInfo fir each entry
	for _, entry := range dirEntries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		entriesPlus = append(entriesPlus, info)
	}

	// Sort the entries by name.
	sort.Sort(sortedEntries(entriesPlus))

	return entriesPlus, nil
}
