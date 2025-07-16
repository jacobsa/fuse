package fuse

import (
	"errors"
	"testing"
)

func Test_umountExpectCustomError(t *testing.T) {
	dir := "/dev/fd/42"
	t.Setenv("PATH", "") // Clear PATH to fail unmount with fusermount is not found

	err := unmount(dir)

	if err == nil || !errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Expected: %v, but got: %v", ErrExternallyManagedMountPoint, err)
	}
}

func Test_umountNoCustomError(t *testing.T) {
	dir := "/dev"
	t.Setenv("PATH", "") // Clear PATH to fail unmount with fusermount is not found

	err := unmount(dir)

	if err != nil && errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Not expected custom error.")
	}
}
