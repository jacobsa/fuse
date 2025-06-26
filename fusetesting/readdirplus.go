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
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FileInfo is a custom implementation of the os.FileInfo interface.
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

// Regular Expression for parsing a long listing line
var longListingRegex = regexp.MustCompile(
	`^([dl-][rwxstST-]{9})` + // File Mode (e.g., -rw-r--r--, drwxr-xr-x)
		`\s+(\S+)` + // Number of links
		`\s+(\S+)` + // User
		`\s+(\S+)` + // Group
		`\s+(\d+)` + // Size
		`\s+([A-Za-z]{3})` + // Month
		`\s+(\d{1,2})` + // Day
		`\s+(\d{2}:\d{2}|\d{4})` + // Time (HH:MM) or Year (YYYY)
		`\s+(.+)$`, // File Name
)

// parseFileMode converts a permission string (e.g., "-rw-r--r--") to fs.FileMode.
func parseFileMode(permissionStr string) (fs.FileMode, error) {
	if len(permissionStr) != 10 {
		return 0, fmt.Errorf("invalid permission string length: %q", permissionStr)
	}

	var fileMode fs.FileMode

	// Set directory or symbolic link bit based on first character.
	switch permissionStr[0] {
	case 'd':
		fileMode |= fs.ModeDir
	case 'l':
		fileMode |= fs.ModeSymlink
	case '-':
		// Regular file; no additional mode bits needed for type.
	default:
		return 0, fmt.Errorf("unknown file type character: %c in %q", permissionStr[0], permissionStr)
	}

	// Parse the 9 permission bits.
	for i, r := range permissionStr[1:] {
		if r != '-' {
			// Calculate the permission bit (e.g., 'r' at index 1 maps to 0o400).
			fileMode |= (1 << (8 - i))
		}
	}
	return fileMode, nil
}

// ParseLine parses a single line of long listing output and returns a FileInfo struct.
func parseLine(line string) (fs.FileInfo, error) {
	match := longListingRegex.FindStringSubmatch(line)
	if len(match) == 0 {
		return nil, fmt.Errorf("invalid line format, no match found for: %q", line)
	}

	permissionStr := match[1]
	sizeStr := match[5]
	fileName := match[9]

	// Parse File Mode
	fileMode, err := parseFileMode(permissionStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file mode from %q: %w", permissionStr, err)
	}

	// Parse File Size
	fileSize, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file size %q: %w", sizeStr, err)
	}

	return FileInfo{
		name: fileName,
		size: fileSize,
		mode: fileMode,
	}, nil
}

// ReadDirPlusPicky executes long listing command on the given directory and parses its output
// into a sorted list of os.FileInfo objects.
func ReadDirPlusPicky(dirname string) (entries []os.FileInfo, err error) {
	cmd := exec.Command("ls", "-l", dirname)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stdout pipe for ls command: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ls command: %w", err)
	}

	// Read the output line by line.
	scanner := bufio.NewScanner(stdout)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			errMsg := strings.ToLower(strings.TrimSpace(stderr.String()))
			return nil, fmt.Errorf("ls failed with message %q: %w", errMsg, err)
		}
		return nil, fmt.Errorf("waiting for ls command to finish: %w", err)
	}

	// Skip the "total" line often included in the output.
	if len(lines) > 0 && strings.HasPrefix(lines[0], "total") {
		lines = lines[1:]
	}

	// Iterate through each line of the output and parse it.
	for _, line := range lines {
		if line == "" {
			continue // Skip empty lines.
		}
		entry, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line %q: %w", line, err)
		}
		entries = append(entries, entry)
	}

	// Sort the collected FileInfo entries by their name.
	sort.Sort(sortedEntries(entries))

	return entries, nil
}
