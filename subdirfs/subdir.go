package subdirfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/src-d/go-billy.v2"
)

// ErrSymlinkNotSupported is returned by Symlink() and Readfile() if the
// underlying filesystem does not support symlinking.
var ErrSymlinkNotSupported = errors.New("symlink not supported")

// SubDir is a helper to implement billy.Filesystem.Dir in other filesystems.
type SubDir struct {
	underlying billy.Filesystem
	base       string
}

// New creates a new filesystem wrapping up the given 'fs'.
// The created filesystem has its base in the given subdirectory of the
// underlying filesystem.
func New(fs billy.Filesystem, base string) billy.Filesystem {
	return &SubDir{fs, base}
}

func (fs *SubDir) underlyingPath(filename string) string {
	return fs.Join(fs.Base(), filename)
}

func (fs *SubDir) Create(filename string) (billy.File, error) {
	f, err := fs.underlying.Create(fs.underlyingPath(filename))
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *SubDir) Open(filename string) (billy.File, error) {
	f, err := fs.underlying.Open(fs.underlyingPath(filename))
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *SubDir) OpenFile(filename string, flag int, mode os.FileMode) (
	billy.File, error) {

	f, err := fs.underlying.OpenFile(fs.underlyingPath(filename), flag, mode)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *SubDir) TempFile(dir, prefix string) (billy.File, error) {
	f, err := fs.underlying.TempFile(fs.underlyingPath(dir), prefix)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, fs.Join(dir, filepath.Base(f.Filename()))), nil
}

func (fs *SubDir) Rename(from, to string) error {
	return fs.underlying.Rename(fs.underlyingPath(from), fs.underlyingPath(to))
}

func (fs *SubDir) Remove(path string) error {
	return fs.underlying.Remove(fs.underlyingPath(path))
}

func (fs *SubDir) MkdirAll(filename string, perm os.FileMode) error {
	fullpath := fs.Join(fs.base, filename)
	return fs.underlying.MkdirAll(fullpath, perm)
}

func (fs *SubDir) Stat(filename string) (billy.FileInfo, error) {
	fullpath := fs.underlyingPath(filename)
	fi, err := fs.underlying.Stat(fullpath)
	if err != nil {
		return nil, err
	}

	return newFileInfo(filepath.Base(fullpath), fi), nil
}

func (fs *SubDir) Lstat(filename string) (billy.FileInfo, error) {
	fullpath := fs.underlyingPath(filename)
	fi, err := fs.underlying.Lstat(fullpath)
	if err != nil {
		return nil, err
	}

	return newFileInfo(filepath.Base(fullpath), fi), nil
}

func (fs *SubDir) ReadDir(path string) ([]billy.FileInfo, error) {
	prefix := fs.underlyingPath(path)
	fis, err := fs.underlying.ReadDir(prefix)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(fis); i++ {
		rn := strings.Replace(fis[i].Name(), prefix, "", 1)
		fis[i] = newFileInfo(rn, fis[i])
	}

	return fis, nil
}

func (fs *SubDir) Join(elem ...string) string {
	return fs.underlying.Join(elem...)
}

func (fs *SubDir) Dir(path string) billy.Filesystem {
	return New(fs.underlying, fs.underlyingPath(path))
}

func (fs *SubDir) Base() string {
	return fs.base
}

func (fs *SubDir) Symlink(target, link string) error {
	target = filepath.FromSlash(target)

	// only rewrite target if it's already absolute
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		target = string(os.PathSeparator) + fs.underlyingPath(target)
	}

	link = fs.underlyingPath(link)
	return fs.underlying.Symlink(target, link)
}

func (fs *SubDir) Readlink(link string) (string, error) {
	fullpath := fs.underlyingPath(link)
	target, err := fs.underlying.Readlink(fullpath)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(target) && !strings.HasPrefix(target, string(filepath.Separator)) {
		return target, nil
	}

	base := string(os.PathSeparator) + fs.base
	target, err = filepath.Rel(base, target)
	if err != nil {
		return "", err
	}

	return string(os.PathSeparator) + target, nil
}
