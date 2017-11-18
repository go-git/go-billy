// +build !windows

package osfs

import (
	"os"
	"syscall"
)

func (fs *OS) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return syscall.Flock(int(f.File.Fd()), syscall.LOCK_EX)
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return syscall.Flock(int(f.File.Fd()), syscall.LOCK_UN)
}
