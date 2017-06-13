package chroot 

import (
	"io"
	"path/filepath"

	"gopkg.in/src-d/go-billy.v2"
)

type file struct {
	name string
	f    billy.File
}

func newFile(fs billy.Filesystem, f billy.File, filename string) billy.File {
	filename = fs.Join(fs.Root(), filename)
	filename, _ = filepath.Rel(fs.Root(), filename)

	return &file{
		name: filename,
		f:    f,
	}
}

func (f *file) Name() string {
	return f.name
}

func (f *file) Read(p []byte) (int, error) {
	return f.f.Read(p)
}

func (f *file) ReadAt(b []byte, off int64) (int, error) {
	rf, ok := f.f.(io.ReaderAt)
	if !ok {
		return 0, billy.ErrNotSupported
	}

	return rf.ReadAt(b, off)
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return f.f.Seek(offset, whence)
}

func (f *file) Write(p []byte) (int, error) {
	return f.f.Write(p)
}

func (f *file) Close() error {
	return f.f.Close()
}
