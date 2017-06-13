package chroot

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

// ChrootHelper is a helper to implement billy.Chroot.
type ChrootHelper struct {
	underlying billy.Filesystem
	base       string
}

// New creates a new filesystem wrapping up the given 'fs'.
// The created filesystem has its base in the given ChrootHelperectory of the
// underlying filesystem.
func New(fs billy.Filesystem, base string) billy.Filesystem {
	return &ChrootHelper{fs, base}
}

func (fs *ChrootHelper) underlyingPath(filename string) (string, error) {
	if isCrossBoundaries(filename) {
		return "", billy.ErrCrossedBoundary
	}

	return fs.Join(fs.Root(), filename), nil
}

func isCrossBoundaries(path string) bool {
	path = filepath.ToSlash(path)
	path = filepath.Clean(path)

	return strings.HasPrefix(path, "..")
}

func (fs *ChrootHelper) Create(filename string) (billy.File, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.Create(fullpath)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *ChrootHelper) Open(filename string) (billy.File, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.Open(fullpath)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *ChrootHelper) OpenFile(filename string, flag int, mode os.FileMode) (billy.File, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.OpenFile(fullpath, flag, mode)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *ChrootHelper) TempFile(dir, prefix string) (billy.File, error) {
	fullpath, err := fs.underlyingPath(dir)
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.TempFile(fullpath, prefix)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, fs.Join(dir, filepath.Base(f.Name()))), nil
}

func (fs *ChrootHelper) Rename(from, to string) error {
	var err error
	from, err = fs.underlyingPath(from)
	if err != nil {
		return err
	}

	to, err = fs.underlyingPath(to)
	if err != nil {
		return err
	}

	return fs.underlying.Rename(from, to)
}

func (fs *ChrootHelper) Remove(path string) error {
	fullpath, err := fs.underlyingPath(path)
	if err != nil {
		return err
	}

	return fs.underlying.Remove(fullpath)
}

func (fs *ChrootHelper) MkdirAll(filename string, perm os.FileMode) error {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return err
	}

	return fs.underlying.MkdirAll(fullpath, perm)
}

func (fs *ChrootHelper) Stat(filename string) (os.FileInfo, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	return fs.underlying.Stat(fullpath)
}

func (fs *ChrootHelper) Lstat(filename string) (os.FileInfo, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	return fs.underlying.Lstat(fullpath)
}

func (fs *ChrootHelper) ReadDir(path string) ([]os.FileInfo, error) {
	fullpath, err := fs.underlyingPath(path)
	if err != nil {
		return nil, err
	}

	return fs.underlying.ReadDir(fullpath)
}

func (fs *ChrootHelper) Join(elem ...string) string {
	return fs.underlying.Join(elem...)
}

func (fs *ChrootHelper) Symlink(target, link string) error {
	target = filepath.FromSlash(target)

	// only rewrite target if it's already absolute
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		target = string(os.PathSeparator) + fs.Join(fs.Root(), target)
	}

	if fs.isTargetOutBounders(link, target) {
		return billy.ErrCrossedBoundary
	}

	link, err := fs.underlyingPath(link)
	if err != nil {
		return err
	}

	return fs.underlying.Symlink(target, link)
}

func (fs *ChrootHelper) isTargetOutBounders(link, target string) bool {
	fulllink := fs.Join(fs.base, link)
	fullpath := fs.Join(filepath.Dir(fulllink), target)
	target, err := filepath.Rel(fs.base, fullpath)
	if err != nil {
		return true
	}

	return isCrossBoundaries(target)
}

func (fs *ChrootHelper) Readlink(link string) (string, error) {
	fullpath, err := fs.underlyingPath(link)
	if err != nil {
		return "", err
	}

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

func (fs *ChrootHelper) Chroot(path string) (billy.Basic, error) {
	fullpath, err := fs.underlyingPath(path)
	if err != nil {
		return nil, err
	}

	return New(fs.underlying, fullpath), nil
}

func (fs *ChrootHelper) Root() string {
	return fs.base
}
