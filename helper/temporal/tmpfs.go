package temporal

import (
	"errors"
	"io"
	"os"
	"path"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/helper/chroot"
)

// Temporal provides billy.TempFile capabilities for filesystems that do not
// support it or where there is a performance penalty in doing so.
type Temporal struct {
	billy.Filesystem

	tmp       billy.Filesystem
	tempFiles map[string]bool
}

// New creates a new filesystem wrapping up 'fs' and using 'tmp' for temporary
// files. Any file created with TempFile will be created in 'tmp'. It will be
// moved to 'fs' if Rename is called on ifs.
func New(fs, tmp billy.Filesystem) *Temporal {
	return &Temporal{
		Filesystem: fs,
		tmp:        tmp,
		tempFiles:  map[string]bool{},
	}
}

func (fs *Temporal) Create(path string) (billy.File, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Create(path)
	}

	return fs.Filesystem.Create(path)
}

func (fs *Temporal) Open(path string) (billy.File, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Open(path)
	}

	return fs.Filesystem.Open(path)

}

func (fs *Temporal) OpenFile(p string, flag int, mode os.FileMode) (
	billy.File, error) {
	if fs.isTmpFile(p) {
		return fs.tmp.OpenFile(p, flag, mode)
	}

	return fs.Filesystem.OpenFile(p, flag, mode)
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
		err := copyPath(fs.tmp, fs.Filesystem, from, to)
		if err != nil {
			return err
		}

		if err := fs.tmp.Remove(from); err != nil {
			return err
		}

		fs.removeTempReference(from)

		return nil
	}

	return fs.Filesystem.Rename(from, to)
}

func (fs *Temporal) Remove(path string) error {
	if fs.isTmpFile(path) {
		if err := fs.tmp.Remove(path); err != nil {
			return err
		}

		fs.removeTempReference(path)

		return nil
	}

	return fs.Filesystem.Remove(path)
}

func (fs *Temporal) Symlink(target, link string) error {
	if fs.isTmpFile(target) {
		return billy.ErrNotSupported
	}

	if fs.isTmpFile(link) {
		return os.ErrExist
	}

	return fs.Filesystem.Symlink(target, link)
}

func (fs *Temporal) Stat(path string) (os.FileInfo, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Stat(path)
	}

	return fs.Filesystem.Stat(path)
}

func (fs *Temporal) Lstat(path string) (os.FileInfo, error) {
	if fs.isTmpFile(path) {
		return fs.tmp.Lstat(path)
	}

	return fs.Filesystem.Lstat(path)
}

func (fs *Temporal) Chroot(path string) (billy.Basic, error) {
	return chroot.New(fs, path), nil
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
func copyPath(src, dst billy.Basic, srcPath, dstPath string) error {
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
