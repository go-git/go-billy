//go:build linux

package osfs

import (
	"errors"
	"io"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func copyFileAccel(in, out *os.File) error {
	if err := copyFileRangeLoop(in, out); err == nil {
		return nil
	} else if !isLinuxUnsupported(err) {
		return err
	}
	if _, err := in.Seek(0, 0); err != nil {
		return err
	}
	if _, err := out.Seek(0, 0); err != nil {
		return err
	}
	if err := out.Truncate(0); err != nil {
		return err
	}
	if err := sendfileLoop(in, out); err == nil {
		return nil
	} else if !isLinuxUnsupported(err) {
		return err
	}
	if _, err := in.Seek(0, 0); err != nil {
		return err
	}
	if _, err := out.Seek(0, 0); err != nil {
		return err
	}
	if err := out.Truncate(0); err != nil {
		return err
	}
	_, err := io.Copy(out, in)
	return err
}

func copyFileRangeLoop(in, out *os.File) error {
	for {
		n, err := unix.CopyFileRange(int(in.Fd()), nil, int(out.Fd()), nil, 1<<20, 0)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

func sendfileLoop(in, out *os.File) error {
	for {
		n, err := unix.Sendfile(int(out.Fd()), int(in.Fd()), nil, 1<<20)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

func isLinuxUnsupported(err error) bool {
	return errors.Is(err, syscall.EXDEV) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.EISDIR)
}
