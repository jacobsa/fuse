package fuse

import (
	"errors"
	"testing"
)

func Test_umountExpectCustomError(t *testing.T) {
	t.Setenv("PATH", "") // Clear PATH to fail unmount with fusermount is not found

	err := unmount("/dev/fd/42")
	if err == nil || !errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Expected: %v, but got: %v", ErrExternallyManagedMountPoint, err)
	}
}

func Test_umountNoCustomError(t *testing.T) {
	t.Setenv("PATH", "") // Clear PATH to fail unmount with fusermount is not found

	err := unmount("/dev")
	if err == nil || errors.Is(err, ErrExternallyManagedMountPoint) {
		t.Errorf("Expected error but not the custom error.")
	}
}
