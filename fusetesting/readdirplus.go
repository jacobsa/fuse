// Copyright 2025 Google Inc. All Rights Reserved.
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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FileInfo is a custom implementation of the os.FileInfo interface.
// It holds file metadata such as name, size, mode, and modification time.
type FileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi FileInfo) Name() string       { return fi.name }
func (fi FileInfo) Size() int64        { return fi.size }
func (fi FileInfo) Mode() os.FileMode  { return fi.mode }
func (fi FileInfo) ModTime() time.Time { return fi.modTime }
func (fi FileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi FileInfo) Sys() interface{}   { return nil }

// sortedEntriesPlus is a type that implements the sort.Interface
// for sorting a slice of os.FileInfo by file name.
type sortedEntriesPlus []os.FileInfo

func (f sortedEntriesPlus) Len() int           { return len(f) }
func (f sortedEntriesPlus) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f sortedEntriesPlus) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// parseLsLine parses a single line of `ls -l` output into an os.FileInfo object.
// Example `ls -l` line format: "-rw-r--r-- 1 user group 123 May 27 10:30 file.txt"
func parseLine(line string) (os.FileInfo, error) {
	fields := strings.Fields(line)
	// Expect at least 9 fields: mode, nlink, user, group, size, month, day, time/year, name
	if len(fields) < 9 {
		return nil, fmt.Errorf("invalid `ls -l` line, not enough fields: %q", line)
	}

	// Parse File Mode
	modeStr := fields[0]
	var mode os.FileMode

	// Determine file type (directory, symlink, or regular file)
	switch modeStr[0] {
	case '-':
		// Regular file; no additional mode bits needed for type.
	case 'd':
		mode |= os.ModeDir // Set directory bit
	case 'l':
		mode |= os.ModeSymlink // Set symbolic link bit
	}

	// Parse permissions (rwx for owner, group, others)
	// The first character (modeStr[0]) is the file type, so we start from index 1.
	for i, r := range modeStr[1:] {
		if r != '-' {
			// Calculate the permission bit.
			// The bits are ordered from left to right (owner read, owner write, owner execute,
			// group read, etc.). A bit shift of `(8 - i)` correctly maps 'r' at index 1 to 0o400,
			// 'w' at index 2 to 0o200, etc.
			//
			// Example:
			// i=0 (modeStr[1]): owner read (0o400) -> 1 << (8-0) = 1 << 8 = 256 (0o400)
			// i=1 (modeStr[2]): owner write (0o200) -> 1 << (8-1) = 1 << 7 = 128 (0o200)
			// ... and so on.
			mode |= (1 << (8 - i))
		}
	}

	// Parse File Size
	size, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing size %q: %w", fields[4], err)
	}

	// Extract File Name
	// The file name can contain spaces, so it's the rest of the fields joined.
	// It starts from the 9th field (index 8).
	name := strings.Join(fields[8:], " ")

	// Return the parsed FileInfo.
	return FileInfo{
		name: name,
		size: size,
		mode: mode,
	}, nil
}

// ReadDirPlusPicky executes the `ls -l` command on the given directory,
// parses its output, and returns a sorted list of os.FileInfo objects.
func ReadDirPlusPicky(dirname string) (entriesPlus []os.FileInfo, err error) {
	// Prepare the `ls -l` command.
	cmd := exec.Command("ls", "-l", dirname)
	var stderr bytes.Buffer // To capture error messages from `ls`.
	cmd.Stderr = &stderr

	// Get a pipe to read stdout from the `ls` command.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stdout pipe for ls command: %w", err)
	}

	// Start the `ls` command.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ls command: %w", err)
	}

	// Read the output of `ls` line by line.
	scanner := bufio.NewScanner(stdout)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Wait for the `ls` command to complete and check for errors.
	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			errMsg := strings.ToLower(strings.TrimSpace(stderr.String()))
			return nil, fmt.Errorf("ls failed with message %q: %w", errMsg, err)
		}
		return nil, fmt.Errorf("waiting for ls command to finish: %w", err)
	}

	// `ls -l` often includes a "total" line at the beginning.
	// If present, skip it as it's not a file entry.
	if len(lines) > 0 && strings.HasPrefix(lines[0], "total") {
		lines = lines[1:]
	}

	// Iterate through each line of `ls` output and parse it.
	for _, line := range lines {
		if line == "" {
			continue // Skip empty lines.
		}
		entry, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line %q: %w", line, err)
		}
		entriesPlus = append(entriesPlus, entry)
	}

	// Sort the collected FileInfo entries by their name.
	sort.Sort(sortedEntriesPlus(entriesPlus))

	return entriesPlus, nil
}
