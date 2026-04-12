//go:build !js

/*
   Copyright 2022 The Flux authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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

// FromRoot creates a new instance of BoundOS from an [os.Root].
// The base dir is set to root.Name(). The provided root is used directly
// for all operations and the caller is responsible for its lifecycle.
// If root is nil, all filesystem operations will fail with an error.
func FromRoot(root *os.Root) billy.Filesystem {
	if root == nil {
		return &BoundOS{rootError: errors.New("root is nil")}
	}
	return &BoundOS{baseDir: root.Name(), root: root}
}

// BoundOS is a fs implementation based on the OS filesystem which relies on
// Go's os.Root. Prefer this fs implementation over ChrootOS.
//
// An [os.Root] is opened once and reused for all operations. When created
// via [FromRoot], the caller is responsible for its lifecycle. When created
// via [New] with [WithBoundOS], the root is opened eagerly and released by
// the garbage collector's finalizer (same as [os.File]).
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descends
//     from the base dir.
//  2. Symlinks don't have their targets modified, and therefore can point
//     to locations outside the base dir or to non-existent paths.
//  3. Operations leading to escapes to outside the [os.Root] location results
//     in [ErrPathEscapesParent].
type BoundOS struct {
	baseDir   string
	root      *os.Root
	rootError error
}

func newBoundOS(d string) billy.Filesystem {
	r, err := os.OpenRoot(d)
	if err != nil {
		return &BoundOS{baseDir: d, rootError: err}
	}
	return &BoundOS{baseDir: d, root: r}
}

func (fs *BoundOS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities | billy.SyncCapability
}

func (fs *BoundOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *BoundOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	name = fs.toRelative(name)

	if flag&os.O_CREATE != 0 {
		if err = fs.createDir(root, name); err != nil {
			return nil, translateError(err, name)
		}
	}

	f, err := root.OpenFile(name, flag, perm)
	if err == nil {
		return &file{File: f}, nil
	}

	// When the open fails because the path escapes the root, the file
	// may be a symlink whose target is an absolute path that actually
	// resides inside the root. Resolve the link and retry.
	if flag&os.O_CREATE == 0 && isPathEscapeError(err) {
		fi, lstatErr := root.Lstat(name)
		if lstatErr == nil && fi.Mode()&gofs.ModeSymlink != 0 {
			if fn, rlErr := root.Readlink(name); rlErr == nil {
				fn = fs.toRelative(fn)
				if f, err = root.OpenFile(fn, flag, perm); err == nil {
					return &file{File: f}, nil
				}
			}
		}
	}

	return nil, translateError(err, name)
}

func (fs *BoundOS) ReadDir(name string) ([]gofs.DirEntry, error) {
	name = fs.toRelative(name)
	if name == "" {
		name = "."
	}

	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	f, err := root.Open(name)
	if err != nil {
		return nil, translateError(err, name)
	}

	e, err := f.ReadDir(-1)
	if err != nil {
		return nil, translateError(err, name)
	}
	return e, nil
}

func (fs *BoundOS) Rename(from, to string) error {
	if from == "." || from == fs.baseDir {
		return billy.ErrBaseDirCannotBeRenamed
	}

	from = fs.toRelative(from)
	to = fs.toRelative(to)

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	// Ensure the target directory exists.
	err = root.MkdirAll(filepath.Dir(to), defaultDirectoryMode)
	if err == nil {
		err = root.Rename(from, to)
	}

	return translateError(err, to)
}

func (fs *BoundOS) MkdirAll(name string, perm gofs.FileMode) error {
	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	// os.Root errors when perm contains bits other than the nine least-significant bits (0o777).
	err = root.MkdirAll(name, perm&0o777)
	return translateError(err, name)
}

func (fs *BoundOS) Open(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *BoundOS) Stat(name string) (os.FileInfo, error) {
	name = fs.toRelative(name)
	if name == "" {
		name = "."
	}

	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	fi, err := root.Stat(name)
	if err != nil {
		return nil, translateError(err, name)
	}

	return fi, nil
}

func (fs *BoundOS) Remove(name string) error {
	if name == "." || name == fs.baseDir {
		return billy.ErrBaseDirCannotBeRemoved
	}

	name = fs.toRelative(name)

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	err = root.Remove(name)
	if err == nil {
		return nil
	}

	return translateError(err, name)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within a .tmp dir.
//
// If dir is outside the bound dir, [ErrPathEscapesParent] is returned.
func (fs *BoundOS) TempFile(dir, prefix string) (billy.File, error) {
	return util.TempFile(fs, dir, prefix)
}

func (fs *BoundOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *BoundOS) RemoveAll(name string) error {
	if name == "." || name == fs.baseDir {
		return billy.ErrBaseDirCannotBeRemoved
	}

	name = fs.toRelative(name)

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	return translateError(root.RemoveAll(name), name)
}

func (fs *BoundOS) Symlink(oldname, newname string) error {
	newname = fs.toRelative(newname)

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	err = fs.createDir(root, newname)
	if err != nil {
		return err
	}

	return translateError(root.Symlink(oldname, newname), newname)
}

func (fs *BoundOS) Lstat(name string) (os.FileInfo, error) {
	name = fs.toRelative(name)

	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	fi, err := root.Lstat(name)
	if err != nil {
		return nil, translateError(err, name)
	}

	return fi, nil
}

func (fs *BoundOS) Readlink(name string) (string, error) {
	name = fs.toRelative(name)

	root, err := fs.fsRoot()
	if err != nil {
		return "", err
	}

	lnk, err := root.Readlink(name)
	if err != nil {
		return "", translateError(err, name)
	}

	return lnk, nil
}

func (fs *BoundOS) Chmod(path string, mode gofs.FileMode) error {
	path = fs.toRelative(path)

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}
	return root.Chmod(path, mode)
}

// Chroot returns a new BoundOS filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	fi, err := root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		err := root.MkdirAll(path, defaultDirectoryMode)
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

	return FromRoot(childRoot), nil
}

// Root returns the current base dir of the billy.Filesystem.
// This is required in order for this implementation to be a drop-in
// replacement for other upstream implementations (e.g. memory and osfs).
func (fs *BoundOS) Root() string {
	return fs.baseDir
}

// toRelative converts an absolute path to a path relative to the base dir.
// If the path is already relative it is returned unchanged.
func (fs *BoundOS) toRelative(name string) string {
	if filepath.IsAbs(name) {
		if rel, err := filepath.Rel(fs.baseDir, name); err == nil {
			return rel
		}
	}
	return name
}

func (fs *BoundOS) createDir(root *os.Root, fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := root.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

// fsRoot returns the [os.Root] to use for filesystem operations.
func (fs *BoundOS) fsRoot() (*os.Root, error) {
	if fs.rootError != nil {
		return nil, fs.rootError
	}
	return fs.root, nil
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
