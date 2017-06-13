// Package osfs provides a billy filesystem for the OS.
package osfs // import "gopkg.in/src-d/go-billy.v2/osfs"

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"strings"

	"gopkg.in/src-d/go-billy.v2"
)

const (
	defaultDirectoryMode = 0755
	defaultCreateMode    = 0666
)

// OS is a filesystem based on the os filesystem.
type OS struct {
	base string
}

// New returns a new OS filesystem.
func New(baseDir string) *OS {
	return &OS{
		base: baseDir,
	}
}

func (fs *OS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *OS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fullpath, err := fs.absolutize(filename)
	if err != nil {
		return nil, err
	}

	if flag&os.O_CREATE != 0 {
		if err := fs.createDir(fullpath); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(fullpath, flag, perm)
	if err != nil {
		return nil, err
	}

	filename, err = filepath.Rel(fs.base, fullpath)
	if err != nil {
		return nil, err
	}

	return newOSFile(filename, f), nil
}

func (fs *OS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

func (fs *OS) ReadDir(path string) ([]os.FileInfo, error) {
	fullpath, err := fs.absolutize(path)
	if err != nil {
		return nil, err
	}

	l, err := ioutil.ReadDir(fullpath)
	if err != nil {
		return nil, err
	}

	var s = make([]os.FileInfo, len(l))
	for i, f := range l {
		s[i] = f
	}

	return s, nil
}

func (fs *OS) Rename(from, to string) error {
	var err error
	from, err = fs.absolutize(from)
	if err != nil {
		return err
	}
	to, err = fs.absolutize(to)
	if err != nil {
		return err
	}

	if err := fs.createDir(to); err != nil {
		return err
	}

	return os.Rename(from, to)
}

func (fs *OS) MkdirAll(path string, perm os.FileMode) error {
	fullpath, err := fs.absolutize(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(fullpath, defaultDirectoryMode)
}

func (fs *OS) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *OS) Remove(filename string) error {
	fullpath, err := fs.absolutize(filename)
	if err != nil {
		return err
	}

	return os.Remove(fullpath)
}

func (fs *OS) TempFile(dir, prefix string) (billy.File, error) {
	fullpath, err := fs.absolutize(dir)
	if err != nil {
		return nil, err
	}

	if err := fs.createDir(fullpath + string(os.PathSeparator)); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile(fullpath, prefix)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, err
	}

	filename, err := filepath.Rel(fs.base, fs.Join(fullpath, s.Name()))
	if err != nil {
		return nil, err
	}

	return newOSFile(filename, f), nil
}

func (fs *OS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *OS) RemoveAll(path string) error {
	fullpath := fs.Join(fs.base, path)
	return os.RemoveAll(fullpath)
}

func (fs *OS) Lstat(filename string) (os.FileInfo, error) {
	fullpath := fs.Join(fs.base, filename)
	return os.Lstat(fullpath)
}

func (fs *OS) Symlink(target, link string) error {
	link, err := fs.absolutize(link)
	if err != nil {
		return err
	}

	target = filepath.FromSlash(target)
	// only rewrite target if it's already absolute
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		target = fs.Join(fs.base, target)
	}

	if fs.isTargetOutBounders(link, target) {
		return billy.ErrCrossedBoundary
	}

	if err := fs.createDir(link); err != nil {
		return err
	}

	return os.Symlink(target, link)
}

func (fs *OS) isTargetOutBounders(link, target string) bool {
	fullpath := filepath.Join(filepath.Dir(link), target)
	target, err := filepath.Rel(fs.base, fullpath)
	if err != nil {
		return true
	}

	return isCrossBoundaries(target)
}

func (fs *OS) Readlink(link string) (string, error) {
	fullpath := fs.Join(fs.base, link)
	target, err := os.Readlink(fullpath)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(target) && !strings.HasPrefix(target, string(filepath.Separator)) {
		return target, nil
	}

	target, err = filepath.Rel(fs.base, target)
	if err != nil {
		return "", err
	}

	return string(os.PathSeparator) + target, nil
}

func (fs *OS) Chroot(path string) (billy.Basic, error) {
	fullpath, err := fs.absolutize(path)
	if err != nil {
		return nil, err
	}

	return New(fullpath), nil
}

func (fs *OS) Root() string {
	return fs.base
}

func (fs *OS) absolutize(relpath string) (string, error) {
	relpath = filepath.ToSlash(relpath)
	relpath = filepath.Clean(relpath)
	if strings.HasPrefix(relpath, "..") {
		return "", billy.ErrCrossedBoundary
	}

	relpath = filepath.FromSlash(relpath)
	return fs.Join(fs.base, relpath), nil
}

// osFile represents a file in the os filesystem
type osFile struct {
	name string
	os.File
}

func newOSFile(filename string, file *os.File) billy.File {
	return &osFile{filename, *file}
}

func (f *osFile) Name() string {
	return f.name
}

func isCrossBoundaries(path string) bool {
	path = filepath.ToSlash(path)
	path = filepath.Clean(path)

	return strings.HasPrefix(path, "..")
}
