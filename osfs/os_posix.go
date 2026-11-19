//go:build solaris || aix

package osfs

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.FcntlFlock(f.File.Fd(), unix.F_SETLK, &unix.Flock_t{
		Type: unix.F_UNLCK,
		Whence: 0,
		Start: 0,
		Len: 0,
	})
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.FcntlFlock(f.File.Fd(), unix.F_SETLK, &unix.Flock_t{
		Type: unix.F_UNLCK,
		Whence: 0,
		Start: 0,
		Len: 0,
	})
}

func (f *file) Sync() error {
	return f.File.Sync()
}

// umask sets umask to a new value, and returns a func which allows the
// caller to reset it back to what it was originally.
func umask(m int) func() {
	old := syscall.Umask(m)
	return func() {
		syscall.Umask(old)
	}
}

// Fd exposes the underlying [os.File.Fd] func, which returns the
// system file descriptor or handle referencing the open file.
func (f *file) Fd() (uintptr, bool) {
	return f.File.Fd(), true
}
