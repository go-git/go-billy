//go:build !js
// +build !js

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
	"io/fs"
	gofs "io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
)

var (
	dotPrefixes = []string{"./", ".\\"}

	// ErrPathEscapesParent represents when an action leads to scaping from the
	// given dir the filesystem is bound to.
	//
	// The upstream version of this Error is not public:
	// https://github.com/golang/go/blob/45d6bc76af641853a0bea31c77912bf9fd52ed79/src/os/file.go#L421
	ErrPathEscapesParent = errors.New("path escapes from parent")
)

// BoundOS is a fs implementation based on the OS filesystem which is bound to
// a base dir.
// Prefer this fs implementation over ChrootOS.
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descends
//     from the base dir.
//  2. Symlinks don't have their targets modified, and therefore can point
//     to locations outside the base dir or to non-existent paths.
//  3. Readlink and Lstat ensures that the link file is located within the base
//     dir, evaluating any symlinks that file or base dir may contain.
type BoundOS struct {
	baseDir   string
	root      *os.Root
	rootError error
}

func newBoundOS(d string) billy.Filesystem {
	r, err := os.OpenRoot(d)
	return &BoundOS{baseDir: d, root: r, rootError: err}
}

func (fs *BoundOS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities & billy.SyncCapability
}

func (fs *BoundOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *BoundOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	root, err := fs.fsRoot()
	if err != nil {
		return nil, err
	}

	// When not creating, read symlink links so that they can be made
	// relative and therefore work.
	if flag&os.O_CREATE == 0 {
		fi, err := fs.root.Lstat(name)
		if err == nil && fi.Mode()&gofs.ModeSymlink != 0 {
			fn, err := root.Readlink(name)
			if err != nil {
				return nil, err
			}
			name = fn
		}
	}

	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return nil, err
		}
		name = fn
	}

	if flag&os.O_CREATE != 0 {
		if err = fs.createDir(root, name); err != nil {
			return nil, translateError(err, name)
		}
	}

	f, err := root.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, nil
}

func (fs *BoundOS) ReadDir(name string) ([]fs.DirEntry, error) {
	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return nil, err
		}
		name = fn
	}

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

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	// Ensure directory the new link will be created exists.
	err = root.MkdirAll(filepath.Dir(to), defaultDirectoryMode)
	if err == nil {
		err = root.Rename(from, to)
	}

	return translateError(err, to)
}

func (fs *BoundOS) MkdirAll(name string, _ fs.FileMode) error {
	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	// os.Root errors when perm contains bits other than the nine least-significant bits (0o777).
	err = root.MkdirAll(name, 0o777)
	return translateError(err, name)
}

func (fs *BoundOS) Open(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *BoundOS) Stat(name string) (os.FileInfo, error) {
	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return nil, err
		}
		name = fn
	}

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

	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return err
		}
		name = fn
	}

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
// If dir is outside the bound dir, [os.ErrPermission] is returned.
func (fs *BoundOS) TempFile(dir, prefix string) (billy.File, error) {
	if filepath.IsAbs(dir) {
		path, err := filepath.Rel(fs.baseDir, dir)
		if err != nil {
			return nil, err
		}
		dir = path
	}

	return util.TempFile(fs, dir, prefix)
}

func (fs *BoundOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *BoundOS) RemoveAll(name string) error {
	if name == "." || name == fs.baseDir {
		return billy.ErrBaseDirCannotBeRemoved
	}

	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return err
		}
		name = fn
	}

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	return root.RemoveAll(name)
}

func (fs *BoundOS) Symlink(oldname, newname string) error {
	if filepath.IsAbs(newname) {
		fn, err := filepath.Rel(fs.baseDir, newname)
		if err != nil {
			return err
		}
		newname = fn
	}

	root, err := fs.fsRoot()
	if err != nil {
		return err
	}

	err = fs.createDir(root, newname)
	if err != nil {
		return err
	}

	return root.Symlink(oldname, newname)
}

func (fs *BoundOS) Lstat(name string) (os.FileInfo, error) {
	if filepath.IsAbs(name) {
		fn, err := filepath.Rel(fs.baseDir, name)
		if err != nil {
			return nil, err
		}
		name = fn
	}

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

// Chroot returns a new BoundOS filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	fi, err := fs.root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		err := fs.root.MkdirAll(path, defaultDirectoryMode)
		if err != nil {
			return nil, fmt.Errorf("failed to auto create dir: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	if fi != nil && !fi.IsDir() {
		return nil, fmt.Errorf("cannot chroot: path is not dir")
	}

	root, err := fs.root.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("unable to chroot: %w", err)
	}

	joined := filepath.Join(fs.baseDir, root.Name())
	return New(joined, WithBoundOS()), nil
}

// Root returns the current base dir of the billy.Filesystem.
// This is required in order for this implementation to be a drop-in
// replacement for other upstream implementations (e.g. memory and osfs).
func (fs *BoundOS) Root() string {
	return fs.baseDir
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

func (fs *BoundOS) fsRoot() (*os.Root, error) {
	return fs.root, fs.rootError
}

func translateError(err error, file string) error {
	if err == nil {
		return nil
	}

	if errors.Unwrap(err).Error() == ErrPathEscapesParent.Error() {
		return fmt.Errorf("%w: %q", ErrPathEscapesParent, file)
	}

	return err
}
