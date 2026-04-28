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
	gofs "io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
)

// BoundOS is a fs implementation based on the OS filesystem which relies on
// Go's [os.Root]. A new [os.Root] is opened and closed for each filesystem
// operation to avoid holding a directory handle open.
//
// For better performance, prefer [RootOS] via [FromRoot] with a
// caller-managed [os.Root].
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descends
//     from the base dir.
//  2. Symlinks don't have their targets modified, and therefore can point
//     to locations outside the base dir or to non-existent paths.
//  3. Operations leading to escapes to outside the [os.Root] location results
//     in [ErrPathEscapesParent].
type BoundOS struct {
	baseDir string
}

func newBoundOS(d string) billy.Filesystem {
	return &BoundOS{baseDir: d}
}

// rootFS opens a temporary [RootOS] and returns a cleanup function that
// closes the underlying [os.Root].
func (fs *BoundOS) rootFS() (*RootOS, func(), error) {
	r, err := os.OpenRoot(fs.baseDir)
	if err != nil {
		return nil, func() {}, err
	}
	return &RootOS{root: r}, func() { r.Close() }, nil
}

func (fs *BoundOS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities | billy.SyncCapability
}

func (fs *BoundOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *BoundOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.OpenFile(name, flag, perm)
}

func (fs *BoundOS) ReadDir(name string) ([]gofs.DirEntry, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.ReadDir(name)
}

func (fs *BoundOS) Rename(from, to string) error {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Rename(from, to)
}

func (fs *BoundOS) MkdirAll(name string, perm gofs.FileMode) error {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.MkdirAll(name, perm)
}

func (fs *BoundOS) Open(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *BoundOS) Stat(name string) (os.FileInfo, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.Stat(name)
}

func (fs *BoundOS) Remove(name string) error {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Remove(name)
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
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.RemoveAll(name)
}

func (fs *BoundOS) Symlink(oldname, newname string) error {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Symlink(oldname, newname)
}

func (fs *BoundOS) Lstat(name string) (os.FileInfo, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.Lstat(name)
}

func (fs *BoundOS) Readlink(name string) (string, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return "", err
	}
	defer cleanup()
	return rfs.Readlink(name)
}

func (fs *BoundOS) Chmod(path string, mode gofs.FileMode) error {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Chmod(path, mode)
}

// Chroot returns a new [BoundOS] filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	childRoot, err := openChildRoot(rfs.root, path)
	if err != nil {
		return nil, err
	}
	defer childRoot.Close()

	return newBoundOS(childRoot.Name()), nil
}

// Root returns the current base dir of the billy.Filesystem.
func (fs *BoundOS) Root() string {
	return fs.baseDir
}
