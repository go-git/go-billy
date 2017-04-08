// Package memfs provides a billy filesystem base on memory.
package memfs // import "gopkg.in/src-d/go-billy.v2/memfs"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/src-d/go-billy.v2"
)

const separator = '/'

// Memory a very convenient filesystem based on memory files
type Memory struct {
	base      string
	s         *storage
	tempCount int
}

//New returns a new Memory filesystem
func New() *Memory {
	return &Memory{
		base: "/",
		s:    &storage{make(map[string]*file, 0)},
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
	fullpath := fs.Join(fs.base, filename)
	f, ok := fs.s.files[fullpath]

	if !ok {
		if !isCreate(flag) {
			return nil, os.ErrNotExist
		}

		fs.s.files[fullpath] = newFile(fs.base, fullpath, perm, flag)
		return fs.s.files[fullpath], nil
	}

	if f.isDir {
		return nil, fmt.Errorf("cannot open directory: %s", filename)
	}

	n := newFile(fs.base, fullpath, perm, flag)
	n.content = f.content

	if isAppend(flag) {
		n.position = int64(n.content.Len())
	}

	if isTruncate(flag) {
		n.content.Truncate()
	}

	return n, nil
}

// Stat returns a billy.FileInfo with the information of the requested file.
func (fs *Memory) Stat(filename string) (billy.FileInfo, error) {
	fullpath := fs.Join(fs.base, filename)

	f, ok := fs.s.files[fullpath]
	if ok && !f.isDir {
		return newFileInfo(fs.base, fullpath, f.mode, fs.s.files[fullpath].content.Len()), nil
	}

	info, err := fs.ReadDir(fullpath)
	if err == nil && len(info) != 0 || f != nil && f.isDir {
		fi := newFileInfo(fs.base, fullpath, 0, len(info))
		fi.isDir = true
		return fi, nil
	}

	return nil, os.ErrNotExist
}

// ReadDir returns a list of billy.FileInfo in the given directory.
func (fs *Memory) ReadDir(base string) (entries []billy.FileInfo, err error) {
	base = fs.Join(fs.base, base)

	appendedDirs := make(map[string]bool, 0)
	for fullpath, f := range fs.s.files {
		if !isInDir(base, fullpath) {
			continue
		}

		fullpath, _ = filepath.Rel(base, fullpath)
		parts := strings.Split(fullpath, string(separator))

		if len(parts) == 1 {
			if f.isDir {
				entries = append(entries, &fileInfo{name: parts[0], isDir: true})
			}

			entries = append(entries, &fileInfo{name: parts[0], mode: f.mode, size: f.content.Len()})
			continue
		}

		if _, ok := appendedDirs[parts[0]]; ok {
			continue
		}

		entries = append(entries, &fileInfo{name: parts[0], mode: f.mode, isDir: true})
		appendedDirs[parts[0]] = true
	}

	return
}

// MkdirAll creates a directory.
func (fs *Memory) MkdirAll(path string, perm os.FileMode) error {
	fullpath := fs.Join(fs.base, path)
	f, ok := fs.s.files[fullpath]
	if ok {
		if !f.isDir {
			return fmt.Errorf("%s is a file", path)
		}

		return nil
	}

	fs.s.files[fullpath] = &file{isDir: true}
	return nil
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

	if _, ok := fs.s.files[from]; !ok {
		return os.ErrNotExist
	}

	fs.s.files[to] = fs.s.files[from]
	fs.s.files[to].BaseFilename = to
	delete(fs.s.files, from)

	return nil
}

// Remove deletes a given file from storage.
func (fs *Memory) Remove(filename string) error {
	fullpath := fs.Join(fs.base, filename)
	if _, ok := fs.s.files[fullpath]; !ok {
		if fs.isDir(fullpath) {
			return fmt.Errorf("directory not empty: %s", filename)
		}

		return os.ErrNotExist
	}

	delete(fs.s.files, fullpath)
	return nil
}

// Join concatenatess part of a path together.
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

// Base returns the base path for the filesystem.
func (fs *Memory) Base() string {
	return fs.base
}

func (fs *Memory) isDir(path string) bool {
	for fpath := range fs.s.files {
		if isInDir(path, fpath) {
			return true
		}
	}

	return false
}

type file struct {
	billy.BaseFile

	content  *content
	position int64
	flag     int
	mode     os.FileMode
	isDir    bool
}

func newFile(base, fullpath string, mode os.FileMode, flag int) *file {
	filename, _ := filepath.Rel(base, fullpath)

	return &file{
		BaseFile: billy.BaseFile{BaseFilename: filename},
		content:  &content{},
		mode:     mode,
		flag:     flag,
	}
}

func (f *file) Read(b []byte) (int, error) {
	n, err := f.ReadAt(b, f.position)
	if err != nil {
		return 0, err
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
	f.position += int64(n)

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
		f.position = int64(f.content.Len()) - offset
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

func (f *file) Open() error {
	f.Closed = false
	return nil
}

type fileInfo struct {
	name  string
	size  int
	mode  os.FileMode
	isDir bool
}

func newFileInfo(base, fullpath string, mode os.FileMode, size int) *fileInfo {
	filename, _ := filepath.Rel(base, fullpath)

	return &fileInfo{
		name: filename,
		mode: mode,
		size: size,
	}
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
	return fi.isDir
}

func (*fileInfo) Sys() interface{} {
	return nil
}

type storage struct {
	files map[string]*file
}

type content struct {
	bytes []byte
}

func (c *content) WriteAt(p []byte, off int64) (int, error) {
	prev := len(c.bytes)
	c.bytes = append(c.bytes[:off], p...)
	if len(c.bytes) < prev {
		c.bytes = c.bytes[:prev]
	}

	return len(p), nil
}

func (c *content) ReadAt(b []byte, off int64) (int, error) {
	size := int64(len(c.bytes))
	if off >= size {
		return 0, io.EOF
	}

	l := int64(len(b))
	if off+l > size {
		l = size - off
	}

	n := copy(b, c.bytes[off:off+l])
	return n, nil
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

func isInDir(dir, other string) bool {
	dir = path.Clean(dir)
	dir = toTrailingSlash(dir)
	other = path.Clean(other)

	return strings.HasPrefix(other, dir)
}

func toTrailingSlash(p string) string {
	if strings.HasSuffix(p, "/") {
		return p
	}

	return p + "/"
}
