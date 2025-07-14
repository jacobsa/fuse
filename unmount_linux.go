package fuse

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func unmount(dir string) error {
	err := fuserunmount(dir)
	if err != nil {
		// Suppress fusermount unmount error for /dev/fuse/N mountpoints
		if strings.HasPrefix(dir, "/dev/fuse/") {
			return nil
		}
	}
	return err
}

func fuserunmount(dir string) error {
	fusermount, err := findFusermount()
	if err != nil {
		return err
	}
	cmd := exec.Command(fusermount, "-u", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			output = bytes.TrimRight(output, "\n")
			return fmt.Errorf("%v: %s", err, output)
		}

		return err
	}
	return nil
}
