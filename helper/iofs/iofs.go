// Package iofs provides an adapter from billy.Filesystem to a the
// standard library io.fs.FS interface.
package iofs

import (
	"io"
	"io/fs"
	"path/filepath"

	billyfs "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/polyfill"
)

// Wrap adapts a billy.Filesystem to a io.fs.FS.
func New(fs billyfs.Basic) fs.FS {
	return &adapterFs{fs: polyfill.New(fs)}
}

type adapterFs struct {
	fs billyfs.Filesystem
}

// Type assertion that adapterFS implements the following interfaces:
var _ fs.FS = (*adapterFs)(nil)
var _ fs.ReadDirFS = (*adapterFs)(nil)
var _ fs.StatFS = (*adapterFs)(nil)
var _ fs.ReadFileFS = (*adapterFs)(nil)

// TODO: implement fs.GlobFS, which will be a fair bit more code.

// Open opens the named file on the underlying FS, implementing fs.FS (returning a file or error).
func (a *adapterFs) Open(name string) (fs.File, error) {
	if name[0] == '/' || name != filepath.Clean(name) {
		// fstest.TestFS explicitly checks that these should return error.
		// MemFS performs the clean internally, so we need to block that here for testing purposes.
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	stat, err := a.fs.Stat(name)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		entries, err := a.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return makeDir(stat, entries), nil
	}
	file, err := a.fs.Open(name)
	return &adapterFile{file: file, info: stat}, err
}

// ReadDir reads the named directory, implementing fs.ReadDirFS (returning a listing or error).
func (a *adapterFs) ReadDir(name string) ([]fs.DirEntry, error) {
	items, err := a.fs.ReadDir(name)
	if err != nil {
		return nil, err
	}
	entries := make([]fs.DirEntry, len(items))
	for i, item := range items {
		entries[i] = fs.FileInfoToDirEntry(item)
	}
	return entries, nil
}

// Stat returns information on the named file, implementing fs.StatFS (returning FileInfo or error).
func (a *adapterFs) Stat(name string) (fs.FileInfo, error) {
	return a.fs.Stat(name)
}

// ReadFile reads the named file and returns its contents, implementing fs.ReadFileFS (returning contents or error).
func (a *adapterFs) ReadFile(name string) ([]byte, error) {
	stat, err := a.fs.Stat(name)
	if err != nil {
		return nil, err
	}
	b := make([]byte, stat.Size())
	file, err := a.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	_, err = file.Read(b)
	return b, err
}

type adapterFile struct {
	file billyfs.File
	info fs.FileInfo
}

var _ fs.File = (*adapterFile)(nil)

// Close closes the file, implementing fs.File (and io.Closer).
func (a *adapterFile) Close() error {
	return a.file.Close()
}

// Read reads bytes from the file, implementing fs.File (and io.Reader).
func (a *adapterFile) Read(b []byte) (int, error) {
	return a.file.Read(b)
}

// Stat returns file information, implementing fs.File (returning FileInfo or error).
func (a *adapterFile) Stat() (fs.FileInfo, error) {
	return a.info, nil
}

type adapterDirFile struct {
	adapterFile
	entries []fs.DirEntry
}

var _ fs.ReadDirFile = (*adapterDirFile)(nil)

func makeDir(stat fs.FileInfo, entries []fs.DirEntry) *adapterDirFile {
	return &adapterDirFile{
		adapterFile: adapterFile{info: stat},
		entries:     entries,
	}
}

// Close closes the directory, implementing fs.File (and io.Closer).
// Subtle: note that this is shadowing adapterFile.Close.
func (a *adapterDirFile) Close() error {
	return nil
}

// ReadDir reads the directory contents, implementing fs.ReadDirFile (returning directory listing or error).
func (a *adapterDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if len(a.entries) == 0 && n > 0 {
		return nil, io.EOF
	}
	if n <= 0 || n > len(a.entries) {
		n = len(a.entries)
	}
	entries := a.entries[:n]
	a.entries = a.entries[n:]
	return entries, nil
}
