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
)

// OS X appears to cap the size of writes to 1 MiB. This constant is also used
// for sizing receive buffers, so make it as small as it can be without
// limiting write sizes.
const maxWrite = 1 << 20

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

func openOSXFUSEDev() (*os.File, error) {
	var f *os.File
	var err error
	for i := uint64(0); ; i++ {
		path := "/dev/osxfuse" + strconv.FormatUint(i, 10)
		f, err = os.OpenFile(path, os.O_RDWR, 0000)
		if os.IsNotExist(err) {
			if i == 0 {
				// not even the first device was found -> fuse is not loaded
				return nil, errNotLoaded
			}

			// we've run out of kernel-provided devices
			return nil, errNoAvail
		}

		if err2, ok := err.(*os.PathError); ok && err2.Err == syscall.EBUSY {
			// try the next one
			continue
		}

		if err != nil {
			return nil, err
		}
		return f, nil
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
		"-o", "iosize="+strconv.FormatUint(maxWrite, 10),
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
