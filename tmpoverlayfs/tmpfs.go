package tmpfs

import (
	"errors"
	"io"
	"os"
	"path"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/subdirfs"
)

type tmpFs struct {
	fs        billy.Filesystem
	tmp       billy.Filesystem
	tempFiles map[string]bool
}

// New creates a new filesystem wrapping up 'fs' and using 'tmp' for temporary
// files. Any file created with TempFile will be created in 'tmp'. It will be
// moved to 'fs' if Rename is called on it.
//
// This is particularly useful to provide TempFile for filesystems that do not
// support it or where there is a performance penalty in doing so.
func New(fs billy.Filesystem, tmp billy.Filesystem) billy.Filesystem {
	return &tmpFs{
		fs:        fs,
		tmp:       tmp,
		tempFiles: map[string]bool{},
	}
}

func (t *tmpFs) Create(path string) (billy.File, error) {
	if t.isTmpFile(path) {
		return t.tmp.Create(path)
	}

	return t.fs.Create(path)
}

func (t *tmpFs) Open(path string) (billy.File, error) {
	if t.isTmpFile(path) {
		return t.tmp.Open(path)
	}

	return t.fs.Open(path)

}

func (t *tmpFs) OpenFile(p string, flag int, mode os.FileMode) (
	billy.File, error) {
	if t.isTmpFile(p) {
		return t.tmp.OpenFile(p, flag, mode)
	}

	return t.fs.OpenFile(p, flag, mode)
}

func (t *tmpFs) ReadDir(p string) ([]billy.FileInfo, error) {
	return t.fs.ReadDir(p)
}

func (t *tmpFs) Join(elem ...string) string {
	return t.fs.Join(elem...)
}

func (t *tmpFs) Dir(p string) billy.Filesystem {
	return subdirfs.New(t, p)
}

func (t *tmpFs) MkdirAll(filename string, perm os.FileMode) error {
	return t.fs.MkdirAll(filename, perm)
}

func (t *tmpFs) Base() string {
	return t.fs.Base()
}

func (t *tmpFs) TempFile(dir string, prefix string) (billy.File, error) {
	tmpFile, err := t.tmp.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}

	t.tempFiles[tmpFile.Filename()] = true

	return tmpFile, nil
}

func (t *tmpFs) Rename(from, to string) error {
	if t.isTmpFile(to) {
		return errors.New("cannot rename to temp file")
	}

	if t.isTmpFile(from) {
		err := copyPath(t.tmp, t.fs, from, to)
		if err != nil {
			return err
		}

		if err := t.tmp.Remove(from); err != nil {
			return err
		}

		t.removeTempReference(from)

		return nil
	}

	return t.fs.Rename(from, to)
}

func (t *tmpFs) Remove(path string) error {
	if t.isTmpFile(path) {
		if err := t.tmp.Remove(path); err != nil {
			return err
		}

		t.removeTempReference(path)

		return nil
	}

	return t.fs.Remove(path)
}

func (t *tmpFs) Stat(path string) (billy.FileInfo, error) {
	if t.isTmpFile(path) {
		return t.tmp.Stat(path)
	}

	return t.fs.Stat(path)
}

func (t *tmpFs) isTmpFile(p string) bool {
	p = path.Clean(p)
	_, ok := t.tempFiles[p]
	return ok
}

func (t *tmpFs) removeTempReference(p string) {
	p = path.Clean(p)
	delete(t.tempFiles, p)
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
