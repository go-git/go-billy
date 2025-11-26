// embedfs exposes an embed.FS as a read-only billy.Filesystem.
package embedfs

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v6"
)

type Embed struct {
	underlying *embed.FS
}

func New(efs *embed.FS) billy.Filesystem {
	fs := &Embed{
		underlying: efs,
	}

	if efs == nil {
		fs.underlying = &embed.FS{}
	}

	return fs
}

func (fs *Embed) Root() string {
	return ""
}

func (fs *Embed) Stat(filename string) (os.FileInfo, error) {
	f, err := fs.underlying.Open(filename)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

func (fs *Embed) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *Embed) OpenFile(filename string, flag int, _ os.FileMode) (billy.File, error) {
	if flag&(os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_RDWR|os.O_EXCL|os.O_TRUNC) != 0 {
		return nil, billy.ErrReadOnly
	}

	f, err := fs.underlying.Open(filename)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return nil, fmt.Errorf("cannot open directory: %s", filename)
	}

	data, err := fs.underlying.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Only load the bytes to memory if the files is needed.
	lazyFunc := func() *bytes.Reader { return bytes.NewReader(data) }
	return toFile(lazyFunc, fi), nil
}

// Join return a path with all elements joined by forward slashes.
//
// This behaviour is OS-agnostic.
func (fs *Embed) Join(elem ...string) string {
	for i, el := range elem {
		if el != "" {
			clean := filepath.Clean(strings.Join(elem[i:], "/"))
			return filepath.ToSlash(clean)
		}
	}
	return ""
}

func (fs *Embed) ReadDir(path string) ([]fs.DirEntry, error) {
	return fs.underlying.ReadDir(path)
}

func (fs *Embed) Capabilities() billy.Capability {
	return billy.ReadCapability | billy.SeekCapability
}

// Chroot is not supported.
//
// Calls will always return billy.ErrNotSupported.
func (fs *Embed) Chroot(_ string) (billy.Filesystem, error) {
	return nil, billy.ErrNotSupported
}

// Lstat is not supported.
//
// Calls will always return billy.ErrNotSupported.
func (fs *Embed) Lstat(_ string) (os.FileInfo, error) {
	return nil, billy.ErrNotSupported
}

// Readlink is not supported.
//
// Calls will always return billy.ErrNotSupported.
func (fs *Embed) Readlink(_ string) (string, error) {
	return "", billy.ErrNotSupported
}

// TempFile is not supported.
//
// Calls will always return billy.ErrNotSupported.
func (fs *Embed) TempFile(_, _ string) (billy.File, error) {
	return nil, billy.ErrNotSupported
}

// Symlink is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (fs *Embed) Symlink(_, _ string) error {
	return billy.ErrReadOnly
}

// Create is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (fs *Embed) Create(_ string) (billy.File, error) {
	return nil, billy.ErrReadOnly
}

// Rename is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (fs *Embed) Rename(_, _ string) error {
	return billy.ErrReadOnly
}

// Remove is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (fs *Embed) Remove(_ string) error {
	return billy.ErrReadOnly
}

// MkdirAll is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (fs *Embed) MkdirAll(_ string, _ os.FileMode) error {
	return billy.ErrReadOnly
}

func toFile(lazy func() *bytes.Reader, fi fs.FileInfo) billy.File {
	return &file{
		lazy: lazy,
		fi:   fi,
	}
}

type file struct {
	lazy   func() *bytes.Reader
	reader *bytes.Reader
	fi     fs.FileInfo
	once   sync.Once
}

func (f *file) loadReader() {
	f.reader = f.lazy()
}

func (f *file) Name() string {
	return f.fi.Name()
}

func (f *file) Read(b []byte) (int, error) {
	f.once.Do(f.loadReader)

	return f.reader.Read(b)
}

func (f *file) ReadAt(b []byte, off int64) (int, error) {
	f.once.Do(f.loadReader)

	return f.reader.ReadAt(b, off)
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	f.once.Do(f.loadReader)

	return f.reader.Seek(offset, whence)
}

func (f *file) Stat() (os.FileInfo, error) {
	return f.fi, nil
}

// Close for embedfs file is a no-op.
func (f *file) Close() error {
	return nil
}

// Truncate is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (f *file) Truncate(_ int64) error {
	return billy.ErrReadOnly
}

// Write is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (f *file) Write(_ []byte) (int, error) {
	return 0, billy.ErrReadOnly
}

// WriteAt is not supported.
//
// Calls will always return billy.ErrReadOnly.
func (f *file) WriteAt([]byte, int64) (int, error) {
	return 0, billy.ErrReadOnly
}
