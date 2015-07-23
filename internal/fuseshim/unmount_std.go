// +build !linux

package fuseshim

import (
	"os"
	"syscall"
)

func unmount(dir string) error {
	err := syscall.Unmount(dir, 0)
	if err != nil {
		err = &os.PathError{Op: "unmount", Path: dir, Err: err}
		return err
	}
	return nil
}
