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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v6"
)

var (
	dotPrefixes   = dotPathPrefixes()
	dotSeparators = dotPathSeparators()
)

func dotPathPrefixes() []string {
	if filepath.Separator == '\\' {
		return []string{"./", ".\\"}
	}
	return []string{"./"}
}

func dotPathSeparators() string {
	if filepath.Separator == '\\' {
		return `/\`
	}
	return `/`
}

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
	baseDir         string
	deduplicatePath bool
}

func newBoundOS(d string, deduplicatePath bool) billy.Filesystem {
	return &BoundOS{baseDir: d, deduplicatePath: deduplicatePath}
}

func (fs *BoundOS) Capabilities() billy.Capability {
	return boundCapabilities()
}

func (fs *BoundOS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *BoundOS) OpenFile(filename string, flag int, perm fs.FileMode) (billy.File, error) {
	name := fs.name(filename)
	filename = fs.expandDot(filename)
	fn, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}

	return openFile(fn, name, flag, perm, fs.createDir)
}

func (fs *BoundOS) ReadDir(path string) ([]fs.DirEntry, error) {
	path = fs.expandDot(path)
	dir, err := fs.abs(path)
	if err != nil {
		return nil, err
	}

	return os.ReadDir(dir)
}

func (fs *BoundOS) Rename(from, to string) error {
	if fs.isBaseDir(from) {
		return billy.ErrBaseDirCannotBeRenamed
	}

	f, err := fs.absNoFollow(from)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(f); err != nil {
		return err
	}

	t, err := fs.absNoFollow(to)
	if err != nil {
		return err
	}

	// MkdirAll for target name.
	if err := fs.createDir(t); err != nil {
		return err
	}

	return os.Rename(f, t)
}

func (fs *BoundOS) MkdirAll(path string, perm fs.FileMode) error {
	path = fs.expandDot(path)
	dir, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, perm)
}

func (fs *BoundOS) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *BoundOS) Stat(filename string) (os.FileInfo, error) {
	name := filepath.Base(fs.name(filename))
	filename = fs.expandDot(filename)
	filename, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	return fileInfo{FileInfo: fi, name: name}, nil
}

func (fs *BoundOS) Remove(filename string) error {
	if fs.isBaseDir(filename) {
		return billy.ErrBaseDirCannotBeRemoved
	}

	fn, err := fs.absNoFollow(filename)
	if err != nil {
		return err
	}
	return os.Remove(fn)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within the OS Temporary dir. If dir is provided
// it must descend from the current base dir.
func (fs *BoundOS) TempFile(dir, prefix string) (billy.File, error) {
	name := ""
	if dir != "" {
		name = fs.name(fs.Join(dir, prefix))
	}
	if dir != "" {
		var err error
		dir = fs.expandDot(dir)
		dir, err = fs.abs(dir)
		if err != nil {
			return nil, err
		}

		_, err = os.Stat(dir)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(dir, defaultDirectoryMode)
			if err != nil {
				return nil, err
			}
		}
	}

	f, err := tempFile(dir, prefix, "")
	if err != nil {
		return nil, err
	}
	if name != "" {
		if osFile, ok := f.(*file); ok {
			osFile.name = fs.Join(filepath.Dir(name), filepath.Base(osFile.File.Name()))
		}
	}
	return f, nil
}

func (fs *BoundOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *BoundOS) RemoveAll(path string) error {
	if fs.isBaseDir(path) {
		return billy.ErrBaseDirCannotBeRemoved
	}

	dir, err := fs.absNoFollow(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (fs *BoundOS) Symlink(target, link string) error {
	link = fs.expandDot(link)
	ln, err := fs.abs(link)
	if err != nil {
		return err
	}
	// MkdirAll for containing dir.
	if err := fs.createDir(ln); err != nil {
		return err
	}
	return os.Symlink(target, ln)
}

func (fs *BoundOS) name(p string) string {
	name := fs.rootRelative(p)
	if name == "" {
		return "."
	}
	return name
}

func (fs *BoundOS) expandDot(p string) string {
	if p == "." {
		return fs.baseDir
	}
	for _, prefix := range dotPrefixes {
		if strings.HasPrefix(p, prefix) {
			p = strings.TrimLeft(strings.TrimPrefix(p, prefix), dotSeparators)
			if p == "" {
				return fs.baseDir
			}
			return p
		}
	}
	return p
}

func (fs *BoundOS) isBaseDir(path string) bool {
	if path == "" || filepath.Clean(path) == "." {
		return true
	}
	path = fs.expandDot(path)
	if filepath.Clean(path) == filepath.Clean(fs.baseDir) {
		return true
	}
	abspath, err := fs.abs(path)
	if err != nil {
		return false
	}
	return filepath.Clean(abspath) == filepath.Clean(fs.baseDir)
}

func (fs *BoundOS) absNoFollow(filename string) (string, error) {
	if fs.baseDir == "" {
		return filepath.Clean(fs.expandDot(filename)), nil
	}

	rel := fs.rootRelative(filename)
	if rel == "" {
		return fs.baseDir, nil
	}

	parent, err := fs.secureJoin(filepath.Dir(rel))
	if err != nil {
		return "", err
	}

	return filepath.Join(parent, filepath.Base(rel)), nil
}

func (fs *BoundOS) rootRelative(filename string) string {
	filename = fs.expandDot(filename)
	filename = filepath.Clean(filename)
	if filepath.Clean(filename) == filepath.Clean(fs.baseDir) {
		return ""
	}
	if filepath.IsAbs(filename) {
		if isLocalToBase(fs.baseDir, filename) {
			rel, _ := filepath.Rel(fs.baseDir, filename)
			return cleanUnderRoot(rel)
		}
		return cleanUnderRoot(filename)
	}
	return cleanUnderRoot(filename)
}

func isLocalToBase(base, name string) bool {
	rel, err := filepath.Rel(base, name)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func cleanUnderRoot(filename string) string {
	vol := filepath.VolumeName(filename)
	filename = filename[len(vol):]
	filename = filepath.Join(string(filepath.Separator), filename)
	return strings.TrimLeft(filename, string(filepath.Separator))
}

func (fs *BoundOS) Lstat(filename string) (os.FileInfo, error) {
	filename, err := fs.absNoFollow(filename)
	if err != nil {
		return nil, err
	}
	return os.Lstat(filename)
}

func (fs *BoundOS) Readlink(link string) (string, error) {
	link, err := fs.absNoFollow(link)
	if err != nil {
		return "", err
	}
	return os.Readlink(link)
}

func (fs *BoundOS) secureJoin(path string) (string, error) {
	if filepath.Separator != '\\' {
		return securejoin.SecureJoin(fs.baseDir, path)
	}
	return securejoin.SecureJoinVFS(fs.baseDir, path, boundOSVFS{})
}

type boundOSVFS struct{}

func (boundOSVFS) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (boundOSVFS) Readlink(name string) (string, error) {
	target, err := os.Readlink(name)
	if err != nil {
		return "", err
	}
	if filepath.Separator == '\\' && filepath.VolumeName(target) == "" && (strings.HasPrefix(target, `\`) || strings.HasPrefix(target, `/`)) {
		if vol := filepath.VolumeName(name); vol != "" {
			return vol + filepath.FromSlash(target), nil
		}
	}
	return target, nil
}

func (fs *BoundOS) Chmod(path string, mode fs.FileMode) error {
	abspath, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.Chmod(abspath, mode)
}

// Chroot returns a new BoundOS filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	joined, err := fs.secureJoin(path)
	if err != nil {
		return nil, err
	}
	return &BoundOS{
		baseDir:         joined,
		deduplicatePath: fs.deduplicatePath,
	}, nil
}

// Root returns the current base dir of the billy.Filesystem.
// This is required in order for this implementation to be a drop-in
// replacement for other upstream implementations (e.g. memory and osfs).
func (fs *BoundOS) Root() string {
	return fs.baseDir
}

func (fs *BoundOS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

// abs transforms filename to an absolute path, taking into account the base dir.
// Relative paths won't be allowed to ascend the base dir, so `../file` will become
// `/working-dir/file`.
//
// Note that if filename is a symlink, the returned address will be the target of the
// symlink.
func (fs *BoundOS) abs(filename string) (string, error) {
	if filename == fs.baseDir {
		filename = string(filepath.Separator)
	}

	path, err := fs.secureJoin(filename)
	if err != nil {
		return "", err
	}

	if fs.deduplicatePath {
		vol := filepath.VolumeName(fs.baseDir)
		dup := filepath.Join(fs.baseDir, fs.baseDir[len(vol):])
		if strings.HasPrefix(path, dup+string(filepath.Separator)) {
			return fs.abs(path[len(dup):])
		}
	}
	return path, nil
}
