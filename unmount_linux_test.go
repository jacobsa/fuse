package fuse

import (
	"errors"
	"testing"
)

func Test_umountExpectCustomError(t *testing.T) {
	dir := "/dev/fd/42"
	fuserunmountMock = func(string) error {
		return errors.New("fusermount path not found")
	}

	err := unmount(dir)

	if err == nil || !errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Expected: %v, but got: %v", ErrExternallyManagedMountPoint, err)
	}
}

func Test_umountNoCustomError(t *testing.T) {
	dir := "/dev"
	fuserunmountMock = func(string) error {
		return errors.New("fusermount path not found")
	}
	err := unmount(dir)

	if err != nil && errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Not expected custom error.")
	}
}
