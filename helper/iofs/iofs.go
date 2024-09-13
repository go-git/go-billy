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
func Wrap(fs billyfs.Basic) fs.FS {
	return &adapterFs{fs: polyfill.New(fs)}
}

type adapterFs struct {
	fs billyfs.Filesystem
}

var _ fs.FS = (*adapterFs)(nil)
var _ fs.ReadDirFS = (*adapterFs)(nil)
var _ fs.StatFS = (*adapterFs)(nil)
var _ fs.ReadFileFS = (*adapterFs)(nil)

// GlobFS would be harder, we don't implement for now.

// Open implements fs.FS.
func (a *adapterFs) Open(name string) (fs.File, error) {
	if name[0] == '/' || name != filepath.Clean(name) {
		// fstest.TestFS explicitly checks that these should return error
		// MemFS is performs the clean internally, so we need to block that here for testing.
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

// ReadDir implements fs.ReadDirFS.
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

// Stat implements fs.StatFS.
func (a *adapterFs) Stat(name string) (fs.FileInfo, error) {
	return a.fs.Stat(name)
}

// ReadFile implements fs.ReadFileFS.
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

// Close implements fs.File.
func (a *adapterFile) Close() error {
	return a.file.Close()
}

// Read implements fs.File.
func (a *adapterFile) Read(b []byte) (int, error) {
	return a.file.Read(b)
}

// Stat implements fs.File.
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

// Close implements fs.File.
// Subtle: note that this is shadowing adapterFile.Close.
func (a *adapterDirFile) Close() error {
	return nil
}

// ReadDir implements fs.ReadDirFile.
func (a *adapterDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if len(a.entries) == 0 && n > 0 {
		return nil, io.EOF
	}
	if n <= 0 {
		n = len(a.entries)
	}
	if n > len(a.entries) {
		n = len(a.entries)
	}
	entries := a.entries[:n]
	a.entries = a.entries[n:]
	return entries, nil
}