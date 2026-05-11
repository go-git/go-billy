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
//  1. Read and write operations can only be directed to files which descend
//     from the base dir.
//  2. Symlink targets are stored verbatim and not rewritten, so they may
//     point outside the base dir or to non-existent paths. [BoundOS.Readlink]
//     returns the stored target with path separators normalised to forward
//     slashes (see [filepath.ToSlash]).
//  3. Operations leading to escapes to outside the [os.Root] location result
//     in [ErrPathEscapesParent].
type BoundOS struct {
	baseDir string
}

func newBoundOS(d string) billy.Filesystem {
	if d == "" {
		d = string(os.PathSeparator)
	}
	d = hostPath(d)
	return &BoundOS{baseDir: d}
}

// rootFS opens a temporary [RootOS] and returns a cleanup function that
// closes the underlying [os.Root].
func (fs *BoundOS) rootFS() (*RootOS, func(), error) {
	return fs.rootFSWithCreate("", false)
}

func (fs *BoundOS) rootFSWithCreate(name string, createBase bool) (*RootOS, func(), error) {
	rootPath := fs.operationRoot(name)
	r, err := openRootAt(rootPath, createBase)
	if err != nil {
		return nil, func() {}, err
	}
	return &RootOS{root: r}, func() { r.Close() }, nil
}

func (fs *BoundOS) Capabilities() billy.Capability {
	return boundCapabilities()
}

func (fs *BoundOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *BoundOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	rfs, cleanup, err := fs.rootFSWithCreate(name, flag&os.O_CREATE != 0)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.OpenFile(name, flag, perm)
}

func (fs *BoundOS) ReadDir(name string) ([]gofs.DirEntry, error) {
	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.ReadDir(name)
}

func (fs *BoundOS) Rename(from, to string) error {
	rfs, cleanup, err := fs.rootFSWithCreate(from, false)
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Rename(from, to)
}

func (fs *BoundOS) MkdirAll(name string, perm gofs.FileMode) error {
	rfs, cleanup, err := fs.rootFSWithCreate(name, true)
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
	if fs.isBaseDir(name) {
		return fs.baseInfo(false)
	}

	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.Stat(name)
}

func (fs *BoundOS) Remove(name string) error {
	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Remove(name)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within a .tmp dir.
func (fs *BoundOS) TempFile(dir, prefix string) (billy.File, error) {
	return util.TempFile(fs, dir, prefix)
}

func (fs *BoundOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *BoundOS) RemoveAll(name string) error {
	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.RemoveAll(name)
}

func (fs *BoundOS) Symlink(oldname, newname string) error {
	rfs, cleanup, err := fs.rootFSWithCreate(newname, true)
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Symlink(oldname, newname)
}

func (fs *BoundOS) Lstat(name string) (os.FileInfo, error) {
	if fs.isBaseDir(name) {
		return fs.baseInfo(true)
	}

	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.Lstat(name)
}

func (fs *BoundOS) Readlink(name string) (string, error) {
	rfs, cleanup, err := fs.rootFSWithCreate(name, false)
	if err != nil {
		return "", err
	}
	defer cleanup()
	return rfs.Readlink(name)
}

func (fs *BoundOS) Chmod(path string, mode gofs.FileMode) error {
	rfs, cleanup, err := fs.rootFSWithCreate(path, false)
	if err != nil {
		return err
	}
	defer cleanup()
	return rfs.Chmod(path, mode)
}

// Chroot returns a new [BoundOS] filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	if hostPath, ok := fs.hostAbsolutePath(path); ok {
		return newBoundOS(hostPath), nil
	}

	rfs, cleanup, err := fs.rootFS()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newBoundOS(fs.chrootPath(path)), nil
		}
		return nil, err
	}
	defer cleanup()

	rel := rfs.toRelative(path)
	if rel == "" {
		rel = "."
	}
	childRoot, err := rfs.root.OpenRoot(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newBoundOS(fs.chrootPath(path)), nil
		}
		return nil, err
	}
	defer childRoot.Close()

	return newBoundOS(filepath.Clean(childRoot.Name())), nil
}

// Root returns the current base dir of the billy.Filesystem.
func (fs *BoundOS) Root() string {
	return fs.baseDir
}

func (fs *BoundOS) isBaseDir(name string) bool {
	if name == "" {
		return true
	}

	name = hostPath(name)
	if filepath.IsAbs(name) {
		if rel, ok := relativeInsideBase(fs.baseDir, name); ok {
			return cleanUnderRoot(rel) == ""
		}
	}

	return cleanUnderRoot(name) == ""
}

func (fs *BoundOS) chrootPath(path string) string {
	if hostPath, ok := fs.hostAbsolutePath(path); ok {
		return filepath.Clean(hostPath)
	}

	path = hostPath(path)
	if filepath.IsAbs(path) {
		if rel, ok := relativeInsideBase(fs.baseDir, path); ok {
			return filepath.Clean(filepath.Join(fs.baseDir, rel))
		}
	}

	return filepath.Clean(filepath.Join(fs.baseDir, cleanUnderRoot(path)))
}

func (fs *BoundOS) hostAbsolutePath(path string) (string, bool) {
	if fs.baseDir != string(os.PathSeparator) {
		return "", false
	}

	path = hostPath(path)
	if filepath.VolumeName(path) != "" && filepath.IsAbs(path) {
		return path, true
	}

	return "", false
}

func (fs *BoundOS) operationRoot(path string) string {
	if hostPath, ok := fs.hostAbsolutePath(path); ok {
		vol := filepath.VolumeName(hostPath)
		if vol != "" {
			return vol + string(filepath.Separator)
		}
	}

	return fs.baseDir
}

func (fs *BoundOS) baseInfo(lstat bool) (os.FileInfo, error) {
	base := filepath.Clean(fs.baseDir)
	parent := filepath.Dir(base)
	name := filepath.Base(base)

	if parent == base {
		root, err := openRootAt(base, false)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		if lstat {
			return root.Lstat(".")
		}
		return root.Stat(".")
	}

	root, err := openRootAt(parent, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if lstat {
		return root.Lstat(name)
	}
	return root.Stat(name)
}

func openRootAt(path string, create bool) (*os.Root, error) {
	path = filepath.Clean(path)
	root, err := os.OpenRoot(path)
	if err == nil || !create || !errors.Is(err, os.ErrNotExist) {
		return root, err
	}

	ancestor, rel, err := openExistingAncestor(path)
	if err != nil {
		return nil, err
	}
	defer ancestor.Close()

	if rel != "." {
		if err := ancestor.MkdirAll(rel, defaultDirectoryMode); err != nil {
			return nil, err
		}
	}

	return os.OpenRoot(path)
}

func openExistingAncestor(path string) (*os.Root, string, error) {
	path = filepath.Clean(path)
	ancestorPath := path

	for {
		root, err := os.OpenRoot(ancestorPath)
		if err == nil {
			rel, relErr := filepath.Rel(ancestorPath, path)
			if relErr != nil {
				root.Close()
				return nil, "", relErr
			}
			return root, rel, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}

		parent := filepath.Dir(ancestorPath)
		if parent == ancestorPath {
			return nil, "", err
		}
		ancestorPath = parent
	}
}
