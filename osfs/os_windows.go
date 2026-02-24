//go:build windows

package osfs

import (
	"io/fs"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL    = windows.NewLazySystemDLL("kernel32.dll")
	lockFileExProc = kernel32DLL.NewProc("LockFileEx")
	unlockFileProc = kernel32DLL.NewProc("UnlockFile")
)

const (
	lockfileExclusiveLock = 0x2
)

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	var overlapped windows.Overlapped
	// err is always non-nil as per sys/windows semantics.
	ret, _, err := lockFileExProc.Call(f.File.Fd(), lockfileExclusiveLock, 0, 0xFFFFFFFF, 0,
		uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if ret == 0 {
		return err
	}
	return nil
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	// err is always non-nil as per sys/windows semantics.
	ret, _, err := unlockFileProc.Call(f.File.Fd(), 0, 0, 0xFFFFFFFF, 0)
	if ret == 0 {
		return err
	}
	return nil
}

func (f *file) Sync() error {
	return f.File.Sync()
}

func rename(from, to string) error {
	// On Windows, os.Rename() fails when a read-only file is specified as the
	// destination. (On Linux, for example, it succeeds even if the file is
	// read-only if you have write permission in the parent directory.)
	// Therefore, for read-only files, we must first change the permissions to
	// allow writing, then rename them with os.Rename(), and then restore their
	// original permissions.
	var (
		modeChanged  bool
		originalMode fs.FileMode
	)
	if fi, err := os.Stat(to); err == nil {
		originalMode = fi.Mode()
		if originalMode&0o200 == 0 {
			err := os.Chmod(to, originalMode|0o200)
			if err != nil {
				return err
			}
			modeChanged = true
		}
	}
	err := os.Rename(from, to)
	if err != nil {
		return err
	}
	// If we changed permissions, change them back
	if modeChanged {
		return os.Chmod(to, originalMode)
	}
	return nil
}

func umask(_ int) func() {
	return func() {
	}
}
