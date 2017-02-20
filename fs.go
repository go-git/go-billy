package billy

import (
	"errors"
	"io"
	"os"
)

var (
	ErrClosed       = errors.New("file: Writing on closed file.")
	ErrReadOnly     = errors.New("this is a read-only filesystem")
	ErrNotSupported = errors.New("feature not supported")
)

// Filesystem abstract the operations in a storage-agnostic interface.
// It allows you to:
// * Create files.
// * Open existing files.
// * Get info about files.
// * List files in a directory.
// * Get a temporal file.
// * Rename files.
// * Remove files.
// * Create directories.
// * Join parts of path.
// * Obtain a filesystem starting on a subdirectory in the current filesystem.
// * Get the base path for the filesystem.
// Each method implementation varies from implementation to implementation. Refer to
// the specific documentation for more info.
type Filesystem interface {
	Create(filename string) (File, error)
	Open(filename string) (File, error)
	OpenFile(filename string, flag int, perm os.FileMode) (File, error)
	Stat(filename string) (FileInfo, error)
	ReadDir(path string) ([]FileInfo, error)
	TempFile(dir, prefix string) (File, error)
	Rename(from, to string) error
	Remove(filename string) error
	MkdirAll(filename string, perm os.FileMode) error
	Join(elem ...string) string
	Dir(path string) Filesystem
	Base() string
}

// File implements io.Closer, io.Reader, io.Seeker, and io.Writer>
// Provides method to obtain the file name and the state of the file (open or closed).
type File interface {
	Filename() string
	IsClosed() bool
	io.Writer
	io.Reader
	io.Seeker
	io.Closer
}

type FileInfo os.FileInfo

type BaseFile struct {
	BaseFilename string
	Closed       bool
}

func (f *BaseFile) Filename() string {
	return f.BaseFilename
}

func (f *BaseFile) IsClosed() bool {
	return f.Closed
}

type removerAll interface {
	RemoveAll(string) error
}

// RemoveAll removes path and any children it contains.
// It removes everything it can but returns the first error
// it encounters. If the path does not exist, RemoveAll
// returns nil (no error).
func RemoveAll(fs Filesystem, path string) error {
	r, ok := fs.(removerAll)
	if ok {
		return r.RemoveAll(path)
	}

	return removeAll(fs, path)
}

func removeAll(fs Filesystem, path string) error {
	// This implementation is adapted from os.RemoveAll.

	// Simple case: if Remove works, we're done.
	err := fs.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// Otherwise, is this a directory we need to recurse into?
	dir, serr := fs.Stat(path)
	if serr != nil {
		if os.IsNotExist(serr) {
			return nil
		}

		return serr
	}

	if !dir.IsDir() {
		// Not a directory; return the error from Remove.
		return err
	}

	// Directory.
	fis, err := fs.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Race. It was deleted between the Lstat and Open.
			// Return nil per RemoveAll's docs.
			return nil
		}

		return err
	}

	// Remove contents & return first error.
	err = nil
	for _, fi := range fis {
		cpath := fs.Join(path, fi.Name())
		err1 := removeAll(fs, cpath)
		if err == nil {
			err = err1
		}
	}

	// Remove directory.
	err1 := fs.Remove(path)
	if err1 == nil || os.IsNotExist(err1) {
		return nil
	}

	if err == nil {
		err = err1
	}

	return err

}
