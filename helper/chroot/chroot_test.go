package chroot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.Create("bar/qux")
	require.NoError(t, err)
	assert.Equal(t, f.Name(), filepath.Join("bar", "qux"))

	assert.Len(t, m.CreateArgs, 1)
	assert.Equal(t, m.CreateArgs[0], "/foo/bar/qux")
}

func TestCreateErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Create("../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestLeadingPeriodsPathNotCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.Create("..foo")
	require.NoError(t, err)
	assert.Equal(t, f.Name(), "..foo")
}

func TestOpen(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.Open("bar/qux")
	require.NoError(t, err)
	assert.Equal(t, f.Name(), filepath.Join("bar", "qux"))

	assert.Len(t, m.OpenArgs, 1)
	assert.Equal(t, m.OpenArgs[0], "/foo/bar/qux")
}

func TestChroot(t *testing.T) {
	m := &test.BasicMock{}

	fs, _ := New(m, "/foo").Chroot("baz")
	f, err := fs.Open("bar/qux")
	require.NoError(t, err)
	assert.Equal(t, f.Name(), filepath.Join("bar", "qux"))

	assert.Len(t, m.OpenArgs, 1)
	assert.Equal(t, m.OpenArgs[0], "/foo/baz/bar/qux")
}

func TestChrootErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs, err := New(m, "/foo").Chroot("../qux")
	assert.Nil(t, fs)
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestOpenErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Open("../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestOpenFile(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	f, err := fs.OpenFile("bar/qux", 42, 0o777)
	require.NoError(t, err)
	assert.Equal(t, f.Name(), filepath.Join("bar", "qux"))

	assert.Len(t, m.OpenFileArgs, 1)
	assert.Equal(t, m.OpenFileArgs[0], [3]interface{}{"/foo/bar/qux", 42, os.FileMode(0o777)})
}

func TestOpenFileErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.OpenFile("../foo", 42, 0o777)
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestStat(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Stat("bar/qux")
	require.NoError(t, err)

	assert.Len(t, m.StatArgs, 1)
	assert.Equal(t, m.StatArgs[0], "/foo/bar/qux")
}

func TestStatErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Stat("../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestRename(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.Rename("bar/qux", "qux/bar")
	require.NoError(t, err)

	assert.Len(t, m.RenameArgs, 1)
	assert.Equal(t, m.RenameArgs[0], [2]string{"/foo/bar/qux", "/foo/qux/bar"})
}

func TestRenameErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.Rename("../foo", "bar")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)

	err = fs.Rename("foo", "../bar")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestRemove(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.Remove("bar/qux")
	require.NoError(t, err)

	assert.Len(t, m.RemoveArgs, 1)
	assert.Equal(t, m.RemoveArgs[0], "/foo/bar/qux")
}

func TestRemoveErrCrossedBoundary(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.Remove("../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestTempFile(t *testing.T) {
	m := &test.TempFileMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("bar", "qux")
	require.NoError(t, err)

	assert.Len(t, m.TempFileArgs, 1)
	assert.Equal(t, m.TempFileArgs[0], [2]string{"/foo/bar", "qux"})
}

func TestTempFileErrCrossedBoundary(t *testing.T) {
	m := &test.TempFileMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("../foo", "qux")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestTempFileWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.TempFile("", "")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestReadDir(t *testing.T) {
	m := &test.DirMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("bar")
	require.NoError(t, err)

	assert.Len(t, m.ReadDirArgs, 1)
	assert.Equal(t, m.ReadDirArgs[0], "/foo/bar")
}

func TestReadDirErrCrossedBoundary(t *testing.T) {
	m := &test.DirMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestReadDirWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.ReadDir("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestMkDirAll(t *testing.T) {
	m := &test.DirMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("bar", 0o777)
	require.NoError(t, err)

	assert.Len(t, m.MkdirAllArgs, 1)
	assert.Equal(t, m.MkdirAllArgs[0], [2]interface{}{"/foo/bar", os.FileMode(0o777)})
}

func TestMkdirAllErrCrossedBoundary(t *testing.T) {
	m := &test.DirMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("../foo", 0o777)
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestMkdirAllWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.MkdirAll("", 0)
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestLstat(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("qux")
	require.NoError(t, err)

	assert.Len(t, m.LstatArgs, 1)
	assert.Equal(t, m.LstatArgs[0], "/foo/qux")
}

func TestLstatErrCrossedBoundary(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("../qux")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestLstatWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Lstat("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestSymlink(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("../baz", "qux/bar")
	require.NoError(t, err)

	assert.Len(t, m.SymlinkArgs, 1)
	assert.Equal(t, m.SymlinkArgs[0], [2]string{filepath.FromSlash("../baz"), "/foo/qux/bar"})
}

func TestSymlinkWithAbsoluteTarget(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("/bar", "qux/baz")
	require.NoError(t, err)

	assert.Len(t, m.SymlinkArgs, 1)
	assert.Equal(t, m.SymlinkArgs[0], [2]string{filepath.FromSlash("/foo/bar"), "/foo/qux/baz"})
}

func TestSymlinkErrCrossedBoundary(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("qux", "../foo")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestSymlinkWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	err := fs.Symlink("qux", "bar")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestReadlink(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	link, err := fs.Readlink("/qux")
	require.NoError(t, err)
	assert.Equal(t, link, filepath.FromSlash("/qux"))

	assert.Len(t, m.ReadlinkArgs, 1)
	assert.Equal(t, m.ReadlinkArgs[0], "/foo/qux")
}

func TestReadlinkWithRelative(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	link, err := fs.Readlink("qux/bar")
	require.NoError(t, err)
	assert.Equal(t, link, filepath.FromSlash("/qux/bar"))

	assert.Len(t, m.ReadlinkArgs, 1)
	assert.Equal(t, m.ReadlinkArgs[0], "/foo/qux/bar")
}

func TestReadlinkErrCrossedBoundary(t *testing.T) {
	m := &test.SymlinkMock{}

	fs := New(m, "/foo")
	_, err := fs.Readlink("../qux")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestReadlinkWithBasic(t *testing.T) {
	m := &test.BasicMock{}

	fs := New(m, "/foo")
	_, err := fs.Readlink("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestCapabilities(t *testing.T) {
	testCapabilities(t, new(test.BasicMock))
	testCapabilities(t, new(test.OnlyReadCapFs))
	testCapabilities(t, new(test.NoLockCapFs))
}

func testCapabilities(t *testing.T, basic billy.Basic) {
	t.Helper()
	baseCapabilities := billy.Capabilities(basic)

	fs := New(basic, "/foo")
	capabilities := billy.Capabilities(fs)

	assert.Equal(t, capabilities, baseCapabilities)
}
