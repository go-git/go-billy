//go:build !plan9 && !windows && !wasm

package chroot

type FileDescriptor interface {
	Fd() (uintptr, bool)
}

// Fd exposes the underlying [os.File.Fd] func, which returns the
// system file descriptor or handle referencing the open file.
// If the underlying Filesystem does not expose this func,
// the return will always be (0, false).
func (f *file) Fd() (uintptr, bool) {
	fd, ok := f.File.(FileDescriptor)
	if ok {
		return fd.Fd()
	}
	return 0, false
}
