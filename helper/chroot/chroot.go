package chroot

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/helper/polyfill"
)

// ChrootHelper is a helper to implement billy.Chroot. It is not a security
// boundary; callers that need containment should use a filesystem implementation
// that enforces paths at the OS boundary instead.
type ChrootHelper struct { //nolint
	underlying billy.Filesystem
	base       string
}

const maxFollowedSymlinks = 8 // Aligns with POSIX_SYMLOOP_MAX

// New creates a new filesystem wrapping the given 'fs', rooted at base.
// All paths passed to the returned filesystem are joined with base before
// being forwarded to the underlying filesystem.
func New(fs billy.Basic, base string) billy.Filesystem {
	return &ChrootHelper{
		underlying: polyfill.New(fs),
		base:       base,
	}
}

func (fs *ChrootHelper) underlyingPath(filename string) (string, error) {
	if isCrossBoundaries(filename) {
		return "", billy.ErrCrossedBoundary
	}

	return fs.Join(fs.Root(), filename), nil
}

func (fs *ChrootHelper) followedPath(filename string, followFinal bool, op string) (string, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return "", err
	}

	sl, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return fullpath, nil
	}

	rel, err := fs.relativeToRoot(fullpath)
	if err != nil {
		return "", err
	}

	fullpath, err = fs.resolveFollowedPath(rel, followFinal, op, sl)
	if errors.Is(err, billy.ErrNotSupported) {
		return fs.underlyingPath(filename)
	}

	return fullpath, err
}

func (fs *ChrootHelper) resolveFollowedPath(rel string, followFinal bool, op string, sl billy.Symlink) (string, error) {
	if rel == "" {
		return fs.resolveFollowedRoot(followFinal, op, sl)
	}

	parts := splitRelativePath(rel)
	resolved := ""
	followed := 0

	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]

		currentRel := joinRelativePath(resolved, part)
		currentPath := fs.Join(fs.Root(), currentRel)
		if len(parts) == 0 && !followFinal {
			return currentPath, nil
		}

		fi, err := sl.Lstat(currentPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fs.Join(fs.Root(), joinRelativePath(append([]string{currentRel}, parts...)...)), nil
			}
			return "", err
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			resolved = currentRel
			continue
		}

		followed++
		if followed > maxFollowedSymlinks {
			return "", symlinkLoopError(op, currentPath)
		}

		target, err := sl.Readlink(currentPath)
		if err != nil {
			return "", err
		}

		targetRel, err := fs.linkTargetRel(currentPath, target)
		if err != nil {
			return "", err
		}
		if targetRel == currentRel {
			return "", symlinkLoopError(op, currentPath)
		}

		parts = append(splitRelativePath(targetRel), parts...)
		resolved = ""
	}

	return fs.Join(fs.Root(), resolved), nil
}

func symlinkLoopError(op, path string) error {
	return &os.PathError{Op: op, Path: path, Err: syscall.ELOOP}
}

func (fs *ChrootHelper) resolveFollowedRoot(followFinal bool, op string, sl billy.Symlink) (string, error) {
	root := fs.Join(fs.Root(), "")
	if !followFinal {
		return root, nil
	}

	fi, err := sl.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return root, nil
		}
		return "", err
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		return root, nil
	}

	target, err := sl.Readlink(root)
	if err != nil {
		return "", err
	}

	targetRel, err := fs.linkTargetRel(root, target)
	if err != nil {
		return root, err
	}
	if targetRel == "" {
		return "", symlinkLoopError(op, root)
	}

	return fs.resolveFollowedPath(targetRel, followFinal, op, sl)
}

func (fs *ChrootHelper) relativeToRoot(filename string) (string, error) {
	rel, err := filepath.Rel(filepath.Clean(fs.Root()), filepath.Clean(filename))
	if err != nil || isCrossBoundaries(rel) {
		return "", billy.ErrCrossedBoundary
	}

	if rel == "." {
		return "", nil
	}
	return rel, nil
}

func (fs *ChrootHelper) linkTargetRel(linkPath, target string) (string, error) {
	target = filepath.FromSlash(target)
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		return fs.relativeToRoot(target)
	}

	return fs.relativeToRoot(fs.Join(filepath.Dir(linkPath), target))
}

func splitRelativePath(filename string) []string {
	filename = filepath.Clean(filename)
	if filename == "" || filename == "." {
		return nil
	}

	return strings.Split(filepath.ToSlash(filename), "/")
}

func joinRelativePath(elem ...string) string {
	parts := make([]string, 0, len(elem))
	for _, part := range elem {
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}

	if len(parts) == 0 {
		return ""
	}
	return filepath.Join(parts...)
}

func isCreateExclusive(flag int) bool {
	return flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0
}

func isCrossBoundaries(name string) bool {
	name = filepath.ToSlash(name)
	name = strings.TrimLeft(name, "/")
	name = path.Clean(name)

	return name == ".." || strings.HasPrefix(name, "../")
}

func (fs *ChrootHelper) Create(filename string) (billy.File, error) {
	fullpath, err := fs.followedPath(filename, true, "create")
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
	fullpath, err := fs.followedPath(filename, true, "open")
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.Open(fullpath)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *ChrootHelper) OpenFile(filename string, flag int, mode fs.FileMode) (billy.File, error) {
	fullpath, err := fs.followedPath(filename, !isCreateExclusive(flag), "open")
	if err != nil {
		return nil, err
	}

	f, err := fs.underlying.OpenFile(fullpath, flag, mode)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, filename), nil
}

func (fs *ChrootHelper) Stat(filename string) (os.FileInfo, error) {
	fullpath, err := fs.followedPath(filename, true, "stat")
	if err != nil {
		return nil, err
	}

	fi, err := fs.underlying.Stat(fullpath)
	if err != nil {
		return nil, err
	}
	return fileInfo{FileInfo: fi, name: filepath.Base(filename)}, nil
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

func (fs *ChrootHelper) Join(elem ...string) string {
	return fs.underlying.Join(elem...)
}

func (fs *ChrootHelper) TempFile(dir, prefix string) (billy.File, error) {
	fullpath, err := fs.underlyingPath(dir)
	if err != nil {
		return nil, err
	}

	tf, ok := fs.underlying.(billy.TempFile)
	if !ok {
		return nil, errors.New("underlying fs does not implement billy.TempFile")
	}

	f, err := tf.TempFile(fullpath, prefix)
	if err != nil {
		return nil, err
	}

	return newFile(fs, f, fs.Join(dir, filepath.Base(f.Name()))), nil
}

func (fs *ChrootHelper) ReadDir(path string) ([]fs.DirEntry, error) {
	fullpath, err := fs.followedPath(path, true, "readdir")
	if err != nil {
		return nil, err
	}

	d, ok := fs.underlying.(billy.Dir)
	if !ok {
		return nil, errors.New("underlying fs does not implement billy.Dir")
	}
	return d.ReadDir(fullpath)
}

func (fs *ChrootHelper) MkdirAll(filename string, perm fs.FileMode) error {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return err
	}

	d, ok := fs.underlying.(billy.Dir)
	if !ok {
		return errors.New("underlying fs does not implement billy.Dir")
	}
	return d.MkdirAll(fullpath, perm)
}

func (fs *ChrootHelper) Lstat(filename string) (os.FileInfo, error) {
	fullpath, err := fs.underlyingPath(filename)
	if err != nil {
		return nil, err
	}

	sl, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return nil, errors.New("underlying fs does not implement billy.Symlink")
	}
	return sl.Lstat(fullpath)
}

// Symlink creates link with the given target. Slashes in target are
// normalised to the host separator. Absolute targets are rewritten to be
// rooted under the chroot base before being stored, so that the link is
// resolvable when the chroot is reopened from a different working directory.
// Relative targets are stored as provided.
func (fs *ChrootHelper) Symlink(target, link string) error {
	target = filepath.FromSlash(target)

	// only rewrite target if it's already absolute
	if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
		target = fs.Join(fs.Root(), target)
		target = filepath.Clean(filepath.FromSlash(target))
	}

	link, err := fs.underlyingPath(link)
	if err != nil {
		return err
	}

	sl, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return errors.New("underlying fs does not implement billy.Symlink")
	}
	return sl.Symlink(target, link)
}

// Readlink returns the target stored for link.
//
// Relative targets are returned unchanged, preserving the original separators
// as written by [ChrootHelper.Symlink] or the underlying filesystem. Absolute
// targets that resolve under the chroot base are translated back to be
// absolute relative to the chroot (using the host path separator), so callers
// see paths in the chroot's coordinate system rather than the underlying
// filesystem's. Absolute targets that resolve outside the base are returned
// as written.
func (fs *ChrootHelper) Readlink(link string) (string, error) {
	fullpath, err := fs.underlyingPath(link)
	if err != nil {
		return "", err
	}

	sl, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return "", errors.New("underlying fs does not implement billy.Symlink")
	}

	target, err := sl.Readlink(fullpath)
	if err != nil {
		return "", err
	}

	rawTarget := target
	target = filepath.FromSlash(target)
	if filepath.Separator == '\\' && len(target) >= 3 &&
		target[0] == '\\' && target[2] == ':' &&
		filepath.VolumeName(fs.base) != "" {
		target = target[1:]
	}

	if !filepath.IsAbs(target) && !strings.HasPrefix(target, string(filepath.Separator)) {
		return rawTarget, nil
	}

	target, err = filepath.Rel(fs.base, target)
	if err != nil {
		return "", err
	}

	return string(os.PathSeparator) + target, nil
}

func (fs *ChrootHelper) Chmod(path string, mode fs.FileMode) error {
	fullpath, err := fs.underlyingPath(path)
	if err != nil {
		return err
	}

	c, ok := fs.underlying.(billy.Chmod)
	if !ok {
		return errors.New("underlying fs does not implement billy.Chmod")
	}
	return c.Chmod(fullpath, mode)
}

func (fs *ChrootHelper) Chroot(path string) (billy.Filesystem, error) {
	fullpath, err := fs.underlyingPath(path)
	if err != nil {
		return nil, err
	}

	return New(fs.underlying, fullpath), nil
}

func (fs *ChrootHelper) Root() string {
	return fs.base
}

func (fs *ChrootHelper) Underlying() billy.Basic {
	return fs.underlying
}

// Capabilities implements the Capable interface.
func (fs *ChrootHelper) Capabilities() billy.Capability {
	return billy.Capabilities(fs.underlying)
}

type file struct {
	billy.File
	name string
}

type fileInfo struct {
	os.FileInfo
	name string
}

func newFile(fs billy.Filesystem, f billy.File, filename string) billy.File {
	filename = fs.Join(fs.Root(), filename)
	filename, _ = filepath.Rel(fs.Root(), filename)

	return &file{
		File: f,
		name: filename,
	}
}

func (f *file) Name() string {
	return f.name
}

func (fi fileInfo) Name() string {
	return fi.name
}
