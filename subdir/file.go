package subdir

import (
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/src-d/go-billy.v2"
)

type file struct {
	billy.BaseFile

	f billy.File
}

func newFile(f billy.File, filename string) billy.File {
	return &file{
		BaseFile: billy.BaseFile{BaseFilename: resolve(filename)},
		f:        f,
	}
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
	defer func() { f.Closed = true }()
	return f.f.Close()
}

func resolve(path string) string {
	rp := filepath.Clean(path)
	if rp == "/" {
		rp = "."
	} else if strings.HasPrefix(rp, "/") {
		rp = rp[1:]
	}

	return rp
}
