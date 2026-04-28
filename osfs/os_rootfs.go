//go:build !js

package osfs

import (
	"errors"
	"fmt"
	gofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
)

// ErrPathEscapesParent represents when an action leads to escaping from the
// given dir the filesystem is bound to.
//
// The upstream version of this error used by [os.Root] is not public.
var ErrPathEscapesParent = errors.New("path escapes from parent")

// FromRoot creates a new [RootOS] from an [os.Root].
// The provided root is used directly for all operations and the caller
// is responsible for its lifecycle. Root must not be nil.
func FromRoot(root *os.Root) (*RootOS, error) {
	if root == nil {
		return nil, errors.New("root must not be nil")
	}
	return &RootOS{root: root}, nil
}

// RootOS is a high-performance fs implementation based on a caller-managed
// [os.Root]. Since the root is reused across all operations, this avoids
// the overhead of opening and closing it on every call. The caller is
// responsible for the root's lifecycle.
//
// For automatic lifecycle management at the cost of per-operation overhead,
// use [BoundOS] via [New] with [WithBoundOS] instead.
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descends
//     from the root dir.
//  2. Symlinks don't have their targets modified, and therefore can point
//     to locations outside the root dir or to non-existent paths.
//  3. Operations leading to escapes to outside the [os.Root] location results
//     in [ErrPathEscapesParent].
type RootOS struct {
	root *os.Root
}

func (fs *RootOS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities | billy.SyncCapability
}

func (fs *RootOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *RootOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	name = fs.toRelative(name)

	if flag&os.O_CREATE != 0 {
		if err := createDir(fs.root, name); err != nil {
			return nil, translateError(err, name)
		}
	}

	f, err := fs.root.OpenFile(name, flag, perm)
	if err == nil {
		return &file{File: f}, nil
	}

	// When the open fails because the path escapes the root, the file
	// may be a symlink whose target is an absolute path that actually
	// resides inside the root. Resolve the link and retry.
	if flag&os.O_CREATE == 0 && isPathEscapeError(err) {
		fi, lstatErr := fs.root.Lstat(name)
		if lstatErr == nil && fi.Mode()&gofs.ModeSymlink != 0 {
			if fn, rlErr := fs.root.Readlink(name); rlErr == nil {
				fn = fs.toRelative(fn)
				if f, err = fs.root.OpenFile(fn, flag, perm); err == nil {
					return &file{File: f}, nil
				}
			}
		}
	}

	return nil, translateError(err, name)
}

func (fs *RootOS) ReadDir(name string) ([]gofs.DirEntry, error) {
	name = fs.toRelative(name)
	if name == "" {
		name = "."
	}

	f, err := fs.root.Open(name)
	if err != nil {
		return nil, translateError(err, name)
	}
	defer f.Close()

	e, err := f.ReadDir(-1)
	if err != nil {
		return nil, translateError(err, name)
	}
	return e, nil
}

func (fs *RootOS) Rename(from, to string) error {
	if from == "." || from == fs.Root() {
		return billy.ErrBaseDirCannotBeRenamed
	}

	from = fs.toRelative(from)
	to = fs.toRelative(to)

	// Ensure the target directory exists.
	err := fs.root.MkdirAll(filepath.Dir(to), defaultDirectoryMode)
	if err == nil {
		err = fs.root.Rename(from, to)
	}

	return translateError(err, to)
}

func (fs *RootOS) MkdirAll(name string, perm gofs.FileMode) error {
	// os.Root errors when perm contains bits other than the nine least-significant bits (0o777).
	err := fs.root.MkdirAll(name, perm&0o777)
	return translateError(err, name)
}

func (fs *RootOS) Open(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *RootOS) Stat(name string) (os.FileInfo, error) {
	name = fs.toRelative(name)
	if name == "" {
		name = "."
	}

	fi, err := fs.root.Stat(name)
	if err != nil {
		return nil, translateError(err, name)
	}

	return fi, nil
}

func (fs *RootOS) Remove(name string) error {
	if name == "." || name == fs.Root() {
		return billy.ErrBaseDirCannotBeRemoved
	}

	name = fs.toRelative(name)

	err := fs.root.Remove(name)
	if err == nil {
		return nil
	}

	return translateError(err, name)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within a .tmp dir.
//
// If dir is outside the root dir, [ErrPathEscapesParent] is returned.
func (fs *RootOS) TempFile(dir, prefix string) (billy.File, error) {
	return util.TempFile(fs, dir, prefix)
}

func (fs *RootOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *RootOS) RemoveAll(name string) error {
	if name == "." || name == fs.Root() {
		return billy.ErrBaseDirCannotBeRemoved
	}

	name = fs.toRelative(name)
	return translateError(fs.root.RemoveAll(name), name)
}

func (fs *RootOS) Symlink(oldname, newname string) error {
	newname = fs.toRelative(newname)

	if err := createDir(fs.root, newname); err != nil {
		return err
	}

	return translateError(fs.root.Symlink(oldname, newname), newname)
}

func (fs *RootOS) Lstat(name string) (os.FileInfo, error) {
	name = fs.toRelative(name)

	fi, err := fs.root.Lstat(name)
	if err != nil {
		return nil, translateError(err, name)
	}

	return fi, nil
}

func (fs *RootOS) Readlink(name string) (string, error) {
	name = fs.toRelative(name)

	lnk, err := fs.root.Readlink(name)
	if err != nil {
		return "", translateError(err, name)
	}

	return lnk, nil
}

func (fs *RootOS) Chmod(path string, mode gofs.FileMode) error {
	path = fs.toRelative(path)
	return fs.root.Chmod(path, mode)
}

// Chroot returns a new [RootOS] filesystem, with the root set to the
// result of joining the provided path with the underlying root dir.
func (fs *RootOS) Chroot(path string) (billy.Filesystem, error) {
	childRoot, err := openChildRoot(fs.root, path)
	if err != nil {
		return nil, err
	}

	return &RootOS{root: childRoot}, nil
}

// Root returns the current root dir of the filesystem.
func (fs *RootOS) Root() string {
	return fs.root.Name()
}

func (fs *RootOS) toRelative(name string) string {
	if filepath.IsAbs(name) {
		if rel, err := filepath.Rel(fs.root.Name(), name); err == nil {
			return rel
		}
	}
	return name
}

// openChildRoot validates path and opens a child [os.Root].
func openChildRoot(root *os.Root, path string) (*os.Root, error) {
	fi, err := root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		err = root.MkdirAll(path, defaultDirectoryMode)
		if err != nil {
			return nil, fmt.Errorf("failed to auto create dir: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	if fi != nil && !fi.IsDir() {
		return nil, fmt.Errorf("cannot chroot: path is not dir")
	}

	childRoot, err := root.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("unable to chroot: %w", err)
	}
	return childRoot, nil
}

func createDir(root *os.Root, fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := root.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

func isPathEscapeError(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrPathEscapesParent.Error())
}

func translateError(err error, file string) error {
	if err == nil {
		return nil
	}

	if isPathEscapeError(err) {
		return fmt.Errorf("%w: %q", ErrPathEscapesParent, file)
	}

	return err
}
