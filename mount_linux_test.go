package fuse

import (
	"testing"
)

func Test_parseFuseFd(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		fd, err := parseFuseFd("/dev/fd/42")
		if fd != 42 {
			t.Errorf("expected 42, got %d", fd)
		}
		if err != nil {
			t.Errorf("expected no error, got %#v", err)
		}
	})

	t.Run("negative", func(t *testing.T) {
		fd, err := parseFuseFd("/dev/fd/-42")
		if fd != -1 {
			t.Errorf("expected an invalid fd, got %d", fd)
		}
		if err == nil {
			t.Errorf("expected an error, nil")
		}
	})

	t.Run("not an int", func(t *testing.T) {
		fd, err := parseFuseFd("/dev/fd/3.14159")
		if fd != -1 {
			t.Errorf("expected an invalid fd, got %d", fd)
		}
		if err == nil {
			t.Errorf("expected an error, nil")
		}
	})
}
