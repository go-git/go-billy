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
	"github.com/go-git/go-billy/v6/helper/chroot"
	"github.com/go-git/go-billy/v6/util"
)

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
// use [BoundOS] via [New] instead.
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descend
//     from the root dir.
//  2. Symlink targets are stored verbatim and not rewritten, so they may
//     point outside the root dir or to non-existent paths. [RootOS.Readlink]
//     returns the stored target with path separators normalised to forward
//     slashes (see [filepath.ToSlash]).
//  3. Operations leading to escapes to outside the [os.Root] location result
//     in [ErrPathEscapesParent].
type RootOS struct {
	root *os.Root
}

func (fs *RootOS) Capabilities() billy.Capability {
	return boundCapabilities()
}

func (fs *RootOS) Create(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *RootOS) OpenFile(name string, flag int, perm gofs.FileMode) (billy.File, error) {
	rel := fs.toRelative(name)
	display := fs.displayName(name)

	if flag&os.O_CREATE != 0 {
		if err := createDir(fs.root, rel); err != nil {
			return nil, translateError(err, rel)
		}
	}

	openName := rel
	if openName == "" {
		openName = "."
	}
	f, err := fs.root.OpenFile(openName, flag, perm)
	if err == nil {
		return &file{File: f, name: display}, nil
	}

	// os.Root rejects an absolute symlink target even when it points back
	// inside the same root. Resolve that one case to keep existing Billy
	// symlink behavior without allowing a real root escape.
	if flag&os.O_CREATE == 0 && isPathEscapeError(err) {
		escapeErr := err
		fi, lstatErr := fs.root.Lstat(openName)
		if lstatErr == nil && fi.Mode()&gofs.ModeSymlink != 0 {
			if target, readlinkErr := fs.root.Readlink(openName); readlinkErr == nil {
				if targetRel, ok := fs.absoluteSymlinkTarget(target); ok {
					if f, err = fs.root.OpenFile(targetRel, flag, perm); err == nil {
						return &file{File: f, name: display}, nil
					}
				}
			}
		}
		err = escapeErr
	}

	return nil, translateError(err, rel)
}

func (fs *RootOS) ReadDir(name string) ([]gofs.DirEntry, error) {
	rel := fs.toRelative(name)
	if rel == "" {
		rel = "."
	}

	f, err := fs.root.Open(rel)
	if err != nil {
		return nil, translateError(err, rel)
	}
	defer f.Close()

	e, err := f.ReadDir(-1)
	if err != nil {
		return nil, translateError(err, rel)
	}
	return e, nil
}

func (fs *RootOS) Rename(from, to string) error {
	if fs.isBaseDir(from) {
		return billy.ErrBaseDirCannotBeRenamed
	}

	fromRel := fs.toRelative(from)
	toRel := fs.toRelative(to)

	if err := createDir(fs.root, toRel); err != nil {
		return translateError(err, toRel)
	}

	return translateError(fs.root.Rename(fromRel, toRel), fmt.Sprintf("%s -> %s", fromRel, toRel))
}

func (fs *RootOS) MkdirAll(name string, perm gofs.FileMode) error {
	rel := fs.toRelative(name)
	if rel == "" {
		rel = "."
	}

	// os.Root errors when perm contains bits other than the nine least-significant bits (0o777).
	err := fs.root.MkdirAll(rel, perm&0o777)
	return translateError(err, rel)
}

func (fs *RootOS) Open(name string) (billy.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *RootOS) Stat(name string) (os.FileInfo, error) {
	rel := fs.toRelative(name)
	if rel == "" {
		rel = "."
	}

	fi, err := fs.root.Stat(rel)
	if err != nil {
		return nil, translateError(err, rel)
	}

	return fileInfo{FileInfo: fi, name: filepath.Base(fs.displayName(name))}, nil
}

func (fs *RootOS) Remove(name string) error {
	if fs.isBaseDir(name) {
		return billy.ErrBaseDirCannotBeRemoved
	}

	rel := fs.toRelative(name)
	err := fs.root.Remove(rel)
	if err == nil {
		return nil
	}

	return translateError(err, rel)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within a .tmp dir.
func (fs *RootOS) TempFile(dir, prefix string) (billy.File, error) {
	return util.TempFile(fs, dir, prefix)
}

func (fs *RootOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *RootOS) RemoveAll(name string) error {
	if fs.isBaseDir(name) {
		return billy.ErrBaseDirCannotBeRemoved
	}

	rel := fs.toRelative(name)
	return translateError(fs.root.RemoveAll(rel), rel)
}

func (fs *RootOS) Symlink(oldname, newname string) error {
	newRel := fs.toRelative(newname)

	if err := createDir(fs.root, newRel); err != nil {
		return translateError(err, newRel)
	}

	return translateError(fs.root.Symlink(oldname, newRel), newRel)
}

func (fs *RootOS) Lstat(name string) (os.FileInfo, error) {
	rel := fs.toRelative(name)
	if rel == "" {
		rel = "."
	}

	fi, err := fs.root.Lstat(rel)
	if err != nil {
		return nil, translateError(err, rel)
	}

	return fileInfo{FileInfo: fi, name: filepath.Base(fs.displayName(name))}, nil
}

func (fs *RootOS) Readlink(name string) (string, error) {
	rel := fs.toRelative(name)
	if rel == "" {
		rel = "."
	}

	lnk, err := fs.root.Readlink(rel)
	if err != nil {
		return "", translateError(err, rel)
	}

	return filepath.ToSlash(lnk), nil
}

func (fs *RootOS) Chmod(path string, mode gofs.FileMode) error {
	rel := fs.toRelative(path)
	if rel == "" {
		rel = "."
	}
	return translateError(fs.root.Chmod(rel, mode), rel)
}

// Chroot returns a logical sub-filesystem rooted at the result of joining
// the provided path with the underlying root dir. The returned filesystem
// reuses this [RootOS]'s [os.Root] (no new file descriptor is opened),
// so its lifecycle is tied to the parent root managed by the caller.
//
// If path does not yet exist under the parent root it is created (with
// [defaultDirectoryMode]). If path exists but is not a directory an error
// is returned.
//
// Containment is enforced by the parent [os.Root], not by the chroot
// path: operations cannot escape the parent root, but a symlink within
// the chroot may resolve to a sibling location elsewhere under the
// parent root. For a tighter boundary at the chroot path, open a new
// [os.Root] via [os.OpenRoot] and wrap it with [FromRoot].
func (fs *RootOS) Chroot(path string) (billy.Filesystem, error) {
	rel := fs.toRelative(path)
	if rel == "" {
		rel = "."
	}

	if err := ensureDir(fs.root, rel); err != nil {
		return nil, err
	}

	return chroot.New(fs, filepath.Join(fs.root.Name(), rel)), nil
}

// Root returns the current root dir of the filesystem.
func (fs *RootOS) Root() string {
	return fs.root.Name()
}

func (fs *RootOS) displayName(name string) string {
	name = hostPath(name)
	if filepath.IsAbs(name) && isHostRoot(fs.root.Name()) {
		return filepath.Clean(name)
	}

	rel := fs.toRelative(name)
	if rel == "" {
		return "."
	}
	return rel
}

func (fs *RootOS) isBaseDir(name string) bool {
	rel := fs.toRelative(name)
	return rel == "" || rel == "."
}

func (fs *RootOS) toRelative(name string) string {
	if name == "" {
		return ""
	}

	name = hostPath(name)
	if filepath.IsAbs(name) {
		if rel, ok := relativeInsideBase(fs.root.Name(), name); ok {
			return cleanUnderRoot(rel)
		}
	}

	return cleanUnderRoot(name)
}

func (fs *RootOS) absoluteSymlinkTarget(target string) (string, bool) {
	if !isRootedPath(target) {
		return "", false
	}
	target = hostPath(target)
	rel, ok := relativeInsideBase(fs.root.Name(), target)
	if !ok {
		rel = cleanUnderRoot(target)
	}
	if rel == "" {
		return ".", true
	}
	return cleanUnderRoot(rel), true
}

func relativeInsideBase(base, name string) (string, bool) {
	base = hostPath(base)
	name = hostPath(name)
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(name))
	if err != nil {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func isRootedPath(name string) bool {
	name = hostPath(name)
	if filepath.IsAbs(name) {
		return true
	}
	return strings.HasPrefix(name, string(filepath.Separator))
}

func cleanUnderRoot(name string) string {
	name = hostPath(name)
	vol := filepath.VolumeName(name)
	name = name[len(vol):]
	name = filepath.Join(string(filepath.Separator), name)
	return strings.TrimLeft(name, string(filepath.Separator))
}

func hostPath(name string) string {
	name = filepath.FromSlash(name)
	if filepath.Separator == '\\' && len(name) >= 3 &&
		name[0] == '\\' && name[2] == ':' {
		return name[1:]
	}
	return name
}

func isHostRoot(name string) bool {
	name = filepath.Clean(hostPath(name))
	if !filepath.IsAbs(name) && !strings.HasPrefix(name, string(filepath.Separator)) {
		return false
	}
	return filepath.Dir(name) == name
}

func ensureDir(root *os.Root, path string) error {
	fi, err := root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := root.MkdirAll(path, defaultDirectoryMode); err != nil {
			return fmt.Errorf("failed to auto create dir: %w", translateError(err, path))
		}
		return nil
	}
	if err != nil {
		return translateError(err, path)
	}
	if fi != nil && !fi.IsDir() {
		return fmt.Errorf("cannot chroot: %q is not a dir (mode %s)", path, fi.Mode())
	}
	return nil
}

func createDir(root *os.Root, fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := root.MkdirAll(dir, defaultDirectoryMode); err != nil {
			if errors.Is(err, os.ErrExist) {
				return nil
			}
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
