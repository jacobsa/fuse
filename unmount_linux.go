package fuse

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Just for testing purposes to mock actual fuserunmount function.
var fuserunmountMock = fuserunmount

func unmount(dir string) error {
	err := fuserunmountMock(dir)
	if err != nil {
		// Return custom error for fusermount unmount error for /dev/fd/N mountpoints
		if strings.HasPrefix(dir, "/dev/fd/") {
			return fmt.Errorf("%w: %s", ErrExternallyManagedMountPoint, err)
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
