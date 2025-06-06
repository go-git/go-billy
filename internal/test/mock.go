package test

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/go-git/go-billy/v6"
)

type BasicMock struct {
	CreateArgs   []string
	OpenArgs     []string
	OpenFileArgs [][3]interface{}
	StatArgs     []string
	RenameArgs   [][2]string
	RemoveArgs   []string
	JoinArgs     [][]string
}

func (fs *BasicMock) Create(filename string) (billy.File, error) {
	fs.CreateArgs = append(fs.CreateArgs, filename)
	return &FileMock{name: filename}, nil
}

func (fs *BasicMock) Open(filename string) (billy.File, error) {
	fs.OpenArgs = append(fs.OpenArgs, filename)
	return &FileMock{name: filename}, nil
}

func (fs *BasicMock) OpenFile(filename string, flag int, mode fs.FileMode) (billy.File, error) {
	fs.OpenFileArgs = append(fs.OpenFileArgs, [3]interface{}{filename, flag, mode})
	return &FileMock{name: filename}, nil
}

func (fs *BasicMock) Stat(filename string) (os.FileInfo, error) {
	fs.StatArgs = append(fs.StatArgs, filename)
	return nil, nil
}

func (fs *BasicMock) Rename(target, link string) error {
	fs.RenameArgs = append(fs.RenameArgs, [2]string{target, link})
	return nil
}

func (fs *BasicMock) Remove(filename string) error {
	fs.RemoveArgs = append(fs.RemoveArgs, filename)
	return nil
}

func (fs *BasicMock) Join(elem ...string) string {
	fs.JoinArgs = append(fs.JoinArgs, elem)
	return path.Join(elem...)
}

type TempFileMock struct {
	BasicMock
	TempFileArgs [][2]string
}

func (fs *TempFileMock) TempFile(dir, prefix string) (billy.File, error) {
	fs.TempFileArgs = append(fs.TempFileArgs, [2]string{dir, prefix})
	return &FileMock{name: "/tmp/hardcoded/mock/temp"}, nil
}

type DirMock struct {
	BasicMock
	ReadDirArgs  []string
	MkdirAllArgs [][2]interface{}
}

func (fs *DirMock) ReadDir(path string) ([]os.FileInfo, error) {
	fs.ReadDirArgs = append(fs.ReadDirArgs, path)
	return nil, nil
}

func (fs *DirMock) MkdirAll(filename string, perm fs.FileMode) error {
	fs.MkdirAllArgs = append(fs.MkdirAllArgs, [2]interface{}{filename, perm})
	return nil
}

type SymlinkMock struct {
	BasicMock
	LstatArgs    []string
	SymlinkArgs  [][2]string
	ReadlinkArgs []string
}

func (fs *SymlinkMock) Lstat(filename string) (os.FileInfo, error) {
	fs.LstatArgs = append(fs.LstatArgs, filename)
	return nil, nil
}

func (fs *SymlinkMock) Symlink(target, link string) error {
	fs.SymlinkArgs = append(fs.SymlinkArgs, [2]string{target, link})
	return nil
}

func (fs *SymlinkMock) Readlink(link string) (string, error) {
	fs.ReadlinkArgs = append(fs.ReadlinkArgs, link)
	return filepath.FromSlash(link), nil
}

type FileMock struct {
	name string
	bytes.Buffer
}

func (f *FileMock) Name() string {
	return f.name
}

func (*FileMock) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, nil
}

func (*FileMock) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, nil
}

func (*FileMock) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}

func (*FileMock) Close() error {
	return nil
}

func (*FileMock) Lock() error {
	return nil
}

func (*FileMock) Unlock() error {
	return nil
}

func (*FileMock) Stat() (fs.FileInfo, error) {
	return nil, nil
}

func (*FileMock) Truncate(_ int64) error {
	return nil
}

type OnlyReadCapFs struct {
	BasicMock
}

func (o *OnlyReadCapFs) Capabilities() billy.Capability {
	return billy.ReadCapability
}

type NoLockCapFs struct {
	BasicMock
}

func (o *NoLockCapFs) Capabilities() billy.Capability {
	return billy.WriteCapability |
		billy.ReadCapability |
		billy.ReadAndWriteCapability |
		billy.SeekCapability |
		billy.TruncateCapability
}
