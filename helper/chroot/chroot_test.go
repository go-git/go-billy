package chroot

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"testing"

	"gopkg.in/src-d/go-billy.v2"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&ChrootSuite{})

type ChrootSuite struct{}

func (s *ChrootSuite) TestCreate(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.Create("bar/qux")
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filepath.Join("bar", "qux"))

	c.Assert(m.CreateArgs, HasLen, 1)
	c.Assert(m.CreateArgs[0], Equals, "/foo/bar/qux")
}

func (s *ChrootSuite) TestCreateErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Create("../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestOpen(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.Open("bar/qux")
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filepath.Join("bar", "qux"))

	c.Assert(m.OpenArgs, HasLen, 1)
	c.Assert(m.OpenArgs[0], Equals, "/foo/bar/qux")
}

func (s *ChrootSuite) TestChroot(c *C) {
	m := &BasicMock{}

	fs, _ := New(m, "/foo").Chroot("baz")
	f, err := fs.Open("bar/qux")
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filepath.Join("bar", "qux"))

	c.Assert(m.OpenArgs, HasLen, 1)
	c.Assert(m.OpenArgs[0], Equals, "/foo/baz/bar/qux")
}

func (s *ChrootSuite) TestChrootErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs, err := New(m, "/foo").Chroot("../qux")
	c.Assert(fs, IsNil)
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestOpenErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Open("../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestOpenFile(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.OpenFile("bar/qux", 42, 0777)
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filepath.Join("bar", "qux"))

	c.Assert(m.OpenFileArgs, HasLen, 1)
	c.Assert(m.OpenFileArgs[0], Equals, [3]interface{}{"/foo/bar/qux", 42, os.FileMode(0777)})
}

func (s *ChrootSuite) TestOpenFileErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.OpenFile("../foo", 42, 0777)
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestStat(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Stat("bar/qux")
	c.Assert(err, IsNil)

	c.Assert(m.StatArgs, HasLen, 1)
	c.Assert(m.StatArgs[0], Equals, "/foo/bar/qux")
}

func (s *ChrootSuite) TestStatErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Stat("../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestRename(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.Rename("bar/qux", "qux/bar")
	c.Assert(err, IsNil)

	c.Assert(m.RenameArgs, HasLen, 1)
	c.Assert(m.RenameArgs[0], Equals, [2]string{"/foo/bar/qux", "/foo/qux/bar"})
}

func (s *ChrootSuite) TestRenameErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.Rename("../foo", "bar")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)

	err = fs.Rename("foo", "../bar")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestRemove(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.Remove("bar/qux")
	c.Assert(err, IsNil)

	c.Assert(m.RemoveArgs, HasLen, 1)
	c.Assert(m.RemoveArgs[0], Equals, "/foo/bar/qux")
}

func (s *ChrootSuite) TestRemoveErrCrossedBoundary(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.Remove("../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestTempFile(c *C) {
	m := &TempFileMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("bar", "qux")
	c.Assert(err, IsNil)

	c.Assert(m.TempFileArgs, HasLen, 1)
	c.Assert(m.TempFileArgs[0], Equals, [2]string{"/foo/bar", "qux"})
}

func (s *ChrootSuite) TestTempFileErrCrossedBoundary(c *C) {
	m := &TempFileMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("../foo", "qux")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestTempFileWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("", "")
	c.Assert(err, Equals, billy.ErrNotSupported)
}

func (s *ChrootSuite) TestReadDir(c *C) {
	m := &DirMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("bar")
	c.Assert(err, IsNil)

	c.Assert(m.ReadDirArgs, HasLen, 1)
	c.Assert(m.ReadDirArgs[0], Equals, "/foo/bar")
}

func (s *ChrootSuite) TestReadDirErrCrossedBoundary(c *C) {
	m := &DirMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestReadDirWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("")
	c.Assert(err, Equals, billy.ErrNotSupported)
}

func (s *ChrootSuite) TestMkDirAll(c *C) {
	m := &DirMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("bar", 0777)
	c.Assert(err, IsNil)

	c.Assert(m.MkdirAllArgs, HasLen, 1)
	c.Assert(m.MkdirAllArgs[0], Equals, [2]interface{}{"/foo/bar", os.FileMode(0777)})
}

func (s *ChrootSuite) TestMkdirAllErrCrossedBoundary(c *C) {
	m := &DirMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("../foo", 0777)
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestMkdirAllWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("", 0)
	c.Assert(err, Equals, billy.ErrNotSupported)
}

func (s *ChrootSuite) TestLstat(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("qux")
	c.Assert(err, IsNil)

	c.Assert(m.LstatArgs, HasLen, 1)
	c.Assert(m.LstatArgs[0], Equals, "/foo/qux")
}

func (s *ChrootSuite) TestLstatErrCrossedBoundary(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("../qux")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestLstatWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("")
	c.Assert(err, Equals, billy.ErrNotSupported)
}

func (s *ChrootSuite) TestSymlink(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("../baz", "qux/bar")
	c.Assert(err, IsNil)

	c.Assert(m.SymlinkArgs, HasLen, 1)
	c.Assert(m.SymlinkArgs[0], Equals, [2]string{filepath.FromSlash("../baz"), "/foo/qux/bar"})
}

func (s *ChrootSuite) TestSymlinkWithAbsoluteTarget(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("/bar", "qux/baz")
	c.Assert(err, IsNil)

	c.Assert(m.SymlinkArgs, HasLen, 1)
	c.Assert(m.SymlinkArgs[0], Equals, [2]string{filepath.FromSlash("/foo/bar"), "/foo/qux/baz"})
}

func (s *ChrootSuite) TestSymlinkErrCrossedBoundary(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("qux", "../foo")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestSymlinkWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("", "")
	c.Assert(err, Equals, billy.ErrNotSupported)
}

func (s *ChrootSuite) TestReadlink(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	link, err := fs.Readlink("/qux")
	c.Assert(err, IsNil)
	c.Assert(link, Equals, filepath.FromSlash("/qux"))

	c.Assert(m.ReadlinkArgs, HasLen, 1)
	c.Assert(m.ReadlinkArgs[0], Equals, "/foo/qux")
}

func (s *ChrootSuite) TestReadlinkWithRelative(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	link, err := fs.Readlink("qux/bar")
	c.Assert(err, IsNil)
	c.Assert(link, Equals, filepath.FromSlash("/qux/bar"))

	c.Assert(m.ReadlinkArgs, HasLen, 1)
	c.Assert(m.ReadlinkArgs[0], Equals, "/foo/qux/bar")
}

func (s *ChrootSuite) TestReadlinkErrCrossedBoundary(c *C) {
	m := &SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Readlink("../qux")
	c.Assert(err, Equals, billy.ErrCrossedBoundary)
}

func (s *ChrootSuite) TestReadlinkWithBasic(c *C) {
	m := &BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Readlink("")
	c.Assert(err, Equals, billy.ErrNotSupported)
}

type BasicMock struct {
	CreateArgs   []string
	OpenArgs     []string
	OpenFileArgs [][3]interface{}
	StatArgs     []string
	RenameArgs   [][2]string
	RemoveArgs   []string
}

func (fs *BasicMock) Create(filename string) (billy.File, error) {
	fs.CreateArgs = append(fs.CreateArgs, filename)
	return &FileMock{name: filename}, nil
}

func (fs *BasicMock) Open(filename string) (billy.File, error) {
	fs.OpenArgs = append(fs.OpenArgs, filename)
	return nil, nil
}

func (fs *BasicMock) OpenFile(filename string, flag int, mode os.FileMode) (billy.File, error) {
	fs.OpenFileArgs = append(fs.OpenFileArgs, [3]interface{}{filename, flag, mode})
	return nil, nil
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

func (fs *DirMock) MkdirAll(filename string, perm os.FileMode) error {
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

func (*FileMock) ReadAt(b []byte, off int64) (int, error) {
	return 0, nil
}

func (*FileMock) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (*FileMock) Close() error {
	return nil
}
