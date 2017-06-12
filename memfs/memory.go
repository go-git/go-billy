// Package memfs provides a billy filesystem base on memory.
package memfs // import "gopkg.in/src-d/go-billy.v2/memfs"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/src-d/go-billy.v2"
)

const separator = filepath.Separator

// Memory a very convenient filesystem based on memory files
type Memory struct {
	base string
	s    *storage

	tempCount int
}

//New returns a new Memory filesystem
func New() *Memory {
	return &Memory{
		base: string(separator),
		s:    newStorage(),
	}
}

// Create returns a new file in memory from a given filename.
func (fs *Memory) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Open returns a readonly file from a given name.
func (fs *Memory) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

// OpenFile returns the file from a given name with given flag and permits.
func (fs *Memory) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fullpath := fs.fullpath(filename)
	f, has := fs.s.Get(fullpath)
	if !has {
		if !isCreate(flag) {
			return nil, os.ErrNotExist
		}

		var err error
		f, err = fs.s.New(fullpath, perm, flag)
		if err != nil {
			return nil, err
		}
	} else {
		if target, isLink := fs.resolveIfLink(fullpath, f); isLink {
			return fs.OpenFile(target, flag, perm)
		}
	}

	if f.mode.IsDir() {
		return nil, fmt.Errorf("cannot open directory: %s", filename)
	}

	filename, err := filepath.Rel(fs.base, fullpath)
	if err != nil {
		return nil, err
	}

	return f.Duplicate(filename, perm, flag), nil
}

func (fs *Memory) fullpath(path string) string {
	return clean(fs.Join(fs.base, path))
}

func (fs *Memory) resolveIfLink(fullpath string, f *file) (target string, isLink bool) {
	if !isSymlink(f.mode) {
		return fullpath, false
	}

	target = string(f.content.bytes)
	if !isAbs(target) {
		target = fs.Join(filepath.Dir(fullpath), target)
	}

	return clean(target), true
}

// On Windows OS, IsAbs validates if a path is valid based on if stars with a
// unit (eg.: `C:\`)  to assert that is absolute, but in this mem implementation
// any path starting by `separator` is also considered absolute.
func isAbs(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, string(separator))
}

// Stat returns a billy.FileInfo with the information of the requested file.
func (fs *Memory) Stat(filename string) (billy.FileInfo, error) {
	fullpath := fs.fullpath(filename)
	f, has := fs.s.Get(fullpath)
	if !has {
		return nil, os.ErrNotExist
	}

	fi := f.Stat()

	var err error
	if target, isLink := fs.resolveIfLink(fullpath, f); isLink {
		fi, err = fs.Stat(target)
		if err != nil {
			return nil, err
		}
	}

	// the name of the file should always the name of the stated file, so we
	// overwrite the Stat returned from the storage with it, since the
	// filename may belong to a link.
	fi.(*fileInfo).name = filepath.Base(filename)
	return fi, nil
}

func (fs *Memory) Lstat(filename string) (billy.FileInfo, error) {
	fullpath := fs.fullpath(filename)
	f, has := fs.s.Get(fullpath)
	if !has {
		return nil, os.ErrNotExist
	}

	return f.Stat(), nil
}

// ReadDir returns a list of billy.FileInfo in the given directory.
func (fs *Memory) ReadDir(path string) ([]billy.FileInfo, error) {
	fullpath := fs.fullpath(path)
	if f, has := fs.s.Get(fullpath); has {
		if target, isLink := fs.resolveIfLink(fullpath, f); isLink {
			return fs.ReadDir(target)
		}
	}

	var entries []billy.FileInfo
	for _, f := range fs.s.Children(fullpath) {
		entries = append(entries, f.Stat())
	}

	return entries, nil
}

// MkdirAll creates a directory.
func (fs *Memory) MkdirAll(path string, perm os.FileMode) error {
	fullpath := fs.Join(fs.base, path)

	_, err := fs.s.New(fullpath, perm|os.ModeDir, 0)
	return err
}

var maxTempFiles = 1024 * 4

// TempFile creates a new temporary file.
func (fs *Memory) TempFile(dir, prefix string) (billy.File, error) {
	var fullpath string
	for {
		if fs.tempCount >= maxTempFiles {
			return nil, errors.New("max. number of tempfiles reached")
		}

		fullpath = fs.getTempFilename(dir, prefix)
		if _, ok := fs.s.files[fullpath]; !ok {
			break
		}
	}

	return fs.Create(fullpath)
}

func (fs *Memory) getTempFilename(dir, prefix string) string {
	fs.tempCount++
	filename := fmt.Sprintf("%s_%d_%d", prefix, fs.tempCount, time.Now().UnixNano())
	return fs.Join(fs.base, dir, filename)
}

// Rename moves a the `from` file to the `to` file.
func (fs *Memory) Rename(from, to string) error {
	from = fs.Join(fs.base, from)
	to = fs.Join(fs.base, to)

	return fs.s.Rename(from, to)
}

// Remove deletes a given file from storage.
func (fs *Memory) Remove(filename string) error {
	fullpath := fs.Join(fs.base, filename)
	return fs.s.Remove(fullpath)
}

// Join joins any number of path elements into a single path, adding a Separator if necessary.
func (fs *Memory) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir creates a new memory filesystem whose root is the given path inside the current
// filesystem.
func (fs *Memory) Dir(path string) billy.Filesystem {
	return &Memory{
		base: fs.Join(fs.base, path),
		s:    fs.s,
	}
}

func (fs *Memory) Symlink(target, link string) error {
	fullpath := clean(fs.Join(fs.base, link))

	_, err := fs.Stat(fullpath)
	if err == nil {
		return os.ErrExist
	}

	if !os.IsNotExist(err) {
		return err
	}

	target = clean(target)
	return billy.WriteFile(fs, fullpath, []byte(target), 0777|os.ModeSymlink)
}

func (fs *Memory) Readlink(link string) (string, error) {
	fullpath := fs.fullpath(link)
	f, has := fs.s.Get(fullpath)
	if !has {
		return "", os.ErrNotExist
	}

	if !isSymlink(f.mode) {
		return "", &os.PathError{
			Op:   "readlink",
			Path: fullpath,
			Err:  fmt.Errorf("not a symlink"),
		}
	}

	return string(f.content.bytes), nil
}

// Base returns the base path for the filesystem.
func (fs *Memory) Base() string {
	return fs.base
}

type file struct {
	billy.BaseFile

	content  *content
	position int64
	flag     int
	mode     os.FileMode
}

func (f *file) Read(b []byte) (int, error) {
	n, err := f.ReadAt(b, f.position)
	f.position += int64(n)

	if err == io.EOF && n != 0 {
		err = nil
	}

	return n, err
}

func (f *file) ReadAt(b []byte, off int64) (int, error) {
	if f.IsClosed() {
		return 0, billy.ErrClosed
	}

	if !isReadAndWrite(f.flag) && !isReadOnly(f.flag) {
		return 0, errors.New("read not supported")
	}

	n, err := f.content.ReadAt(b, off)

	return n, err
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	if f.IsClosed() {
		return 0, billy.ErrClosed
	}

	switch whence {
	case io.SeekCurrent:
		f.position += offset
	case io.SeekStart:
		f.position = offset
	case io.SeekEnd:
		f.position = int64(f.content.Len()) + offset
	}

	return f.position, nil
}

func (f *file) Write(p []byte) (int, error) {
	if f.IsClosed() {
		return 0, billy.ErrClosed
	}

	if !isReadAndWrite(f.flag) && !isWriteOnly(f.flag) {
		return 0, errors.New("write not supported")
	}

	n, err := f.content.WriteAt(p, f.position)
	f.position += int64(n)

	return n, err
}

func (f *file) Close() error {
	if f.IsClosed() {
		return errors.New("file already closed")
	}

	f.Closed = true
	return nil
}

func (f *file) Duplicate(filename string, mode os.FileMode, flag int) billy.File {
	new := &file{
		BaseFile: billy.BaseFile{BaseFilename: filename},
		content:  f.content,
		mode:     mode,
		flag:     flag,
	}

	if isAppend(flag) {
		new.position = int64(new.content.Len())
	}

	if isTruncate(flag) {
		new.content.Truncate()
	}

	return new
}

func (f *file) Stat() billy.FileInfo {
	return &fileInfo{
		name: f.Filename(),
		mode: f.mode,
		size: f.content.Len(),
	}
}

type fileInfo struct {
	name string
	size int
	mode os.FileMode
}

func (fi *fileInfo) Name() string {
	return fi.name
}

func (fi *fileInfo) Size() int64 {
	return int64(fi.size)
}

func (fi *fileInfo) Mode() os.FileMode {
	return fi.mode
}

func (*fileInfo) ModTime() time.Time {
	return time.Now()
}

func (fi *fileInfo) IsDir() bool {
	return fi.mode.IsDir()
}

func (*fileInfo) Sys() interface{} {
	return nil
}

func (c *content) Truncate() {
	c.bytes = make([]byte, 0)
}

func (c *content) Len() int {
	return len(c.bytes)
}

func isCreate(flag int) bool {
	return flag&os.O_CREATE != 0
}

func isAppend(flag int) bool {
	return flag&os.O_APPEND != 0
}

func isTruncate(flag int) bool {
	return flag&os.O_TRUNC != 0
}

func isReadAndWrite(flag int) bool {
	return flag&os.O_RDWR != 0
}

func isReadOnly(flag int) bool {
	return flag == os.O_RDONLY
}

func isWriteOnly(flag int) bool {
	return flag&os.O_WRONLY != 0
}

func isSymlink(m os.FileMode) bool {
	return m&os.ModeSymlink != 0
}
