package tmpfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/subdirfs"
)

// Temporal provides billy.TempFile capabilities for filesystems that do not
// support it or where there is a performance penalty in doing so.
type Temporal struct {
	fs        billy.Filesystem
	tmp       billy.Filesystem
	tempFiles map[string]bool
}

// New creates a new filesystem wrapping up 'fs' and using 'tmp' for temporary
// files. Any file created with TempFile will be created in 'tmp'. It will be
// moved to 'fs' if Rename is called on ifs.
func New(fs, tmp billy.Filesystem) *Temporal {
	return &Temporal{
		fs:        fs,
		tmp:       tmp,
		tempFiles: map[string]bool{},
	}
}

func (fs *Temporal) Create(path string) (billy.File, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Create(path)
	}

	return fs.fs.Create(path)
}

func (fs *Temporal) Open(path string) (billy.File, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Open(path)
	}

	return fs.fs.Open(path)

}

func (fs *Temporal) OpenFile(p string, flag int, mode os.FileMode) (
	billy.File, error) {
	if fs.isTmpFile(p) {
		return fs.tmp.OpenFile(p, flag, mode)
	}

	return fs.fs.OpenFile(p, flag, mode)
}

func (fs *Temporal) ReadDir(p string) ([]os.FileInfo, error) {
	return fs.fs.ReadDir(p)
}

func (fs *Temporal) Join(elem ...string) string {
	return fs.fs.Join(elem...)
}

func (fs *Temporal) MkdirAll(filename string, perm os.FileMode) error {
	return fs.fs.MkdirAll(filename, perm)
}

func (fs *Temporal) TempFile(dir string, prefix string) (billy.File, error) {
	tmpFile, err := fs.tmp.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}

	fs.tempFiles[tmpFile.Name()] = true

	return tmpFile, nil
}

func (fs *Temporal) Rename(from, to string) error {
	if fs.isTmpFile(to) {
		return errors.New("cannot rename to temp file")
	}

	if fs.isTmpFile(from) {
		err := copyPath(fs.tmp, fs.fs, from, to)
		if err != nil {
			return err
		}

		if err := fs.tmp.Remove(from); err != nil {
			return err
		}

		fs.removeTempReference(from)

		return nil
	}

	return fs.fs.Rename(from, to)
}

func (fs *Temporal) Remove(path string) error {
	if fs.isTmpFile(path) {
		if err := fs.tmp.Remove(path); err != nil {
			return err
		}

		fs.removeTempReference(path)

		return nil
	}

	return fs.fs.Remove(path)
}

// Symlink creates a symbolic-link from link to targefs. Using a temporal file
// as target is not allowed.
func (fs *Temporal) Symlink(target, link string) error {
	if fs.isTmpFile(target) {
		return fmt.Errorf("links to temporal file are not supported")
	}

	if fs.isTmpFile(link) {
		return os.ErrExist
	}

	return fs.fs.Symlink(target, link)
}

func (fs *Temporal) Readlink(link string) (string, error) {
	return fs.fs.Readlink(link)
}

func (fs *Temporal) Stat(path string) (os.FileInfo, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Stat(path)
	}

	return fs.fs.Stat(path)
}

func (fs *Temporal) Lstat(path string) (os.FileInfo, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Lstat(path)
	}

	return fs.fs.Lstat(path)
}

func (fs *Temporal) Chroot(path string) (billy.Basic, error) {
	return subdirfs.New(fs, path), nil
}

func (fs *Temporal) Root() string {
	return fs.fs.Root()
}

func (fs *Temporal) isTmpFile(p string) bool {
	p = path.Clean(p)
	_, ok := fs.tempFiles[p]
	return ok
}

func (fs *Temporal) removeTempReference(p string) {
	p = path.Clean(p)
	delete(fs.tempFiles, p)
}

// copyPath copies a file across filesystems.
func copyPath(src billy.Filesystem, dst billy.Filesystem,
	srcPath string, dstPath string) error {

	dstFile, err := dst.Create(dstPath)
	if err != nil {
		return err
	}

	srcFile, err := src.Open(srcPath)
	if err != nil {
		return nil
	}

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return nil
	}

	err = dstFile.Close()
	if err != nil {
		_ = srcFile.Close()
		return err
	}

	return srcFile.Close()
}
