package fuse

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/jacobsa/fuse/internal/buffer"
)

var errNoAvail = errors.New("no available fuse devices")
var errNotLoaded = errors.New("osxfusefs is not loaded")

func loadOSXFUSE() error {
	cmd := exec.Command("/Library/Filesystems/osxfusefs.fs/Support/load_osxfusefs")
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func openOSXFUSEDev() (dev *os.File, err error) {
	// Try each device name.
	for i := uint64(0); ; i++ {
		path := fmt.Sprintf("/dev/osxfuse%d", i)
		dev, err = os.OpenFile(path, os.O_RDWR, 0000)
		if os.IsNotExist(err) {
			if i == 0 {
				// Not even the first device was found. Fuse must not be loaded.
				err = errNotLoaded
				return
			}

			// Otherwise we've run out of kernel-provided devices
			err = errNoAvail
			return
		}

		if err2, ok := err.(*os.PathError); ok && err2.Err == syscall.EBUSY {
			// This device is in use; try the next one.
			continue
		}

		return
	}
}

func callMount(dir string, conf *mountConfig, f *os.File, ready chan<- struct{}, errp *error) error {
	bin := "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs"

	for k, v := range conf.options {
		if strings.Contains(k, ",") || strings.Contains(v, ",") {
			// Silly limitation but the mount helper does not
			// understand any escaping. See TestMountOptionCommaError.
			return fmt.Errorf("mount options cannot contain commas on darwin: %q=%q", k, v)
		}
	}
	cmd := exec.Command(
		bin,
		"-o", conf.getOptions(),
		// Tell osxfuse-kext how large our buffer is. It must split
		// writes larger than this into multiple writes.
		//
		// OSXFUSE seems to ignore InitResponse.MaxWrite, and uses
		// this instead.
		"-o", "iosize="+strconv.FormatUint(buffer.MaxWriteSize, 10),
		// refers to fd passed in cmd.ExtraFiles
		"3",
		dir,
	)
	cmd.ExtraFiles = []*os.File{f}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "MOUNT_FUSEFS_CALL_BY_LIB=")
	// TODO this is used for fs typenames etc, let app influence it
	cmd.Env = append(cmd.Env, "MOUNT_FUSEFS_DAEMON_PATH="+bin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			if buf.Len() > 0 {
				output := buf.Bytes()
				output = bytes.TrimRight(output, "\n")
				msg := err.Error() + ": " + string(output)
				err = errors.New(msg)
			}
		}
		*errp = err
		close(ready)
	}()
	return err
}

// Begin the process of mounting at the given directory, returning a connection
// to the kernel. Mounting continues in the background, and is complete when an
// error is written to the supplied channel. The file system may need to
// service the connection in order for mounting to complete.
func mount(
	dir string,
	conf *mountConfig,
	ready chan<- error) (dev *os.File, err error) {
	// Open the device.
	dev, err = openOSXFUSEDev()

	// Special case: we may need to explicitly load osxfuse. Load it, then try
	// again.
	if err == errNotLoaded {
		err = loadOSXFUSE()
		if err != nil {
			err = fmt.Errorf("loadOSXFUSE: %v", err)
			return
		}

		dev, err = openOSXFUSEDev()
	}

	// Propagate errors.
	if err != nil {
		err = fmt.Errorf("openOSXFUSEDev: %v", err)
		return
	}

	// Call the mount binary with the device.
	err = callMount(dir, conf, dev, ready)
	if err != nil {
		dev.Close()
		err = fmt.Errorf("callMount: %v", err)
		return
	}

	return
}
