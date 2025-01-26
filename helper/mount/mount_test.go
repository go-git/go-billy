package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mock struct {
	test.BasicMock
	test.DirMock
	test.SymlinkMock
}

func setup() (helper *Mount, underlying *mock, source *mock) {
	underlying = &mock{
		BasicMock:   test.BasicMock{},
		DirMock:     test.DirMock{},
		SymlinkMock: test.SymlinkMock{},
	}

	source = &mock{
		BasicMock:   test.BasicMock{},
		DirMock:     test.DirMock{},
		SymlinkMock: test.SymlinkMock{},
	}

	helper = New(underlying, "/foo", source)
	return
}

func TestCreate(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.Create("bar/qux")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("bar", "qux"), f.Name())

	assert.Len(t, underlying.CreateArgs, 1)
	assert.Equal(t, filepath.Join("bar", "qux"), underlying.CreateArgs[0])
	assert.Empty(t, source.CreateArgs)
}

func TestCreateMountPoint(t *testing.T) {
	helper, _, _ := setup()
	f, err := helper.Create("foo")
	assert.Nil(t, f)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestCreateInMount(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.Create("foo/bar/qux")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("foo", "bar", "qux"), f.Name())

	assert.Empty(t, underlying.CreateArgs)
	assert.Len(t, source.CreateArgs, 1)
	assert.Equal(t, filepath.Join("bar", "qux"), source.CreateArgs[0])
}

func TestOpen(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.Open("bar/qux")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("bar", "qux"), f.Name())

	assert.Len(t, underlying.OpenArgs, 1)
	assert.Equal(t, filepath.Join("bar", "qux"), underlying.OpenArgs[0])
	assert.Empty(t, source.OpenArgs)
}

func TestOpenMountPoint(t *testing.T) {
	helper, _, _ := setup()
	f, err := helper.Open("foo")
	assert.Nil(t, f)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestOpenInMount(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.Open("foo/bar/qux")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("foo", "bar", "qux"), f.Name())

	assert.Empty(t, underlying.OpenArgs)
	assert.Len(t, source.OpenArgs, 1)
	assert.Equal(t, source.OpenArgs[0], filepath.Join("bar", "qux"))
}

func TestOpenFile(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.OpenFile("bar/qux", 42, 0777)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("bar", "qux"), f.Name())

	assert.Len(t, underlying.OpenFileArgs, 1)
	assert.Equal(t, underlying.OpenFileArgs[0],
		[3]interface{}{filepath.Join("bar", "qux"), 42, os.FileMode(0777)})
	assert.Empty(t, source.OpenFileArgs)
}

func TestOpenFileMountPoint(t *testing.T) {
	helper, _, _ := setup()
	f, err := helper.OpenFile("foo", 42, 0777)
	assert.Nil(t, f)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestOpenFileInMount(t *testing.T) {
	helper, underlying, source := setup()
	f, err := helper.OpenFile("foo/bar/qux", 42, 0777)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("foo", "bar", "qux"), f.Name())

	assert.Empty(t, underlying.OpenFileArgs)
	assert.Len(t, source.OpenFileArgs, 1)
	assert.Equal(t, source.OpenFileArgs[0],
		[3]interface{}{filepath.Join("bar", "qux"), 42, os.FileMode(0777)})
}

func TestStat(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Stat("bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.StatArgs, 1)
	assert.Equal(t, underlying.StatArgs[0], filepath.Join("bar", "qux"))
	assert.Empty(t, source.StatArgs)
}

func TestStatInMount(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Stat("foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.StatArgs)
	assert.Len(t, source.StatArgs, 1)
	assert.Equal(t, source.StatArgs[0], filepath.Join("bar", "qux"))
}

func TestRename(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Rename("bar/qux", "qux")
	require.NoError(t, err)

	assert.Len(t, underlying.RenameArgs, 1)
	assert.Equal(t, underlying.RenameArgs[0], [2]string{"bar/qux", "qux"})
	assert.Empty(t, source.RenameArgs)
}

func TestRenameInMount(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Rename("foo/bar/qux", "foo/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.RenameArgs)
	assert.Len(t, source.RenameArgs, 1)
	assert.Equal(t, source.RenameArgs[0],
		[2]string{filepath.Join("bar", "qux"), "qux"})
}

func TestRenameCross(t *testing.T) {
	underlying := memfs.New()
	source := memfs.New()

	err := util.WriteFile(underlying, "file", []byte("foo"), 0777)
	require.NoError(t, err)

	fs := New(underlying, "/foo", source)
	err = fs.Rename("file", "foo/file")
	require.NoError(t, err)

	_, err = underlying.Stat("file")
	assert.Equal(t, err, os.ErrNotExist)

	_, err = source.Stat("file")
	require.NoError(t, err)

	err = fs.Rename("foo/file", "file")
	require.NoError(t, err)

	_, err = underlying.Stat("file")
	require.NoError(t, err)

	_, err = source.Stat("file")
	assert.Equal(t, err, os.ErrNotExist)
}

func TestRemove(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Remove("bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.RemoveArgs, 1)
	assert.Equal(t, underlying.RemoveArgs[0], filepath.Join("bar", "qux"))
	assert.Empty(t, source.RemoveArgs)
}

func TestRemoveMountPoint(t *testing.T) {
	helper, _, _ := setup()
	err := helper.Remove("foo")
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestRemoveInMount(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Remove("foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.RemoveArgs)
	assert.Len(t, source.RemoveArgs, 1)
	assert.Equal(t, source.RemoveArgs[0], filepath.Join("bar", "qux"))
}

func TestReadDir(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.ReadDir("bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.ReadDirArgs, 1)
	assert.Equal(t, underlying.ReadDirArgs[0], filepath.Join("bar", "qux"))
	assert.Empty(t, source.ReadDirArgs)
}

func TestJoin(t *testing.T) {
	helper, underlying, source := setup()
	helper.Join("foo", "bar")

	assert.Len(t, underlying.JoinArgs, 1)
	assert.Equal(t, underlying.JoinArgs[0], []string{"foo", "bar"})
	assert.Empty(t, source.JoinArgs)
}

func TestReadDirInMount(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.ReadDir("foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.ReadDirArgs)
	assert.Len(t, source.ReadDirArgs, 1)
	assert.Equal(t, source.ReadDirArgs[0], filepath.Join("bar", "qux"))
}

func TestMkdirAll(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.MkdirAll("bar/qux", 0777)
	require.NoError(t, err)

	assert.Len(t, underlying.MkdirAllArgs, 1)
	assert.Equal(t, underlying.MkdirAllArgs[0],
		[2]interface{}{filepath.Join("bar", "qux"), os.FileMode(0777)})
	assert.Empty(t, source.MkdirAllArgs)
}

func TestMkdirAllInMount(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.MkdirAll("foo/bar/qux", 0777)
	require.NoError(t, err)

	assert.Empty(t, underlying.MkdirAllArgs)
	assert.Len(t, source.MkdirAllArgs, 1)
	assert.Equal(t, source.MkdirAllArgs[0],
		[2]interface{}{filepath.Join("bar", "qux"), os.FileMode(0777)})
}

func TestLstat(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Lstat("bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.LstatArgs, 1)
	assert.Equal(t, underlying.LstatArgs[0], filepath.Join("bar", "qux"))
	assert.Empty(t, source.LstatArgs)
}

func TestLstatInMount(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Lstat("foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.LstatArgs)
	assert.Len(t, source.LstatArgs, 1)
	assert.Equal(t, source.LstatArgs[0], filepath.Join("bar", "qux"))
}

func TestSymlink(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Symlink("../baz", "bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.SymlinkArgs, 1)
	assert.Equal(t, underlying.SymlinkArgs[0],
		[2]string{"../baz", filepath.Join("bar", "qux")})
	assert.Empty(t, source.SymlinkArgs)
}

func TestSymlinkCrossMount(t *testing.T) {
	helper, _, _ := setup()
	err := helper.Symlink("../foo", "bar/qux")
	assert.Error(t, err)

	err = helper.Symlink("../foo/qux", "bar/qux")
	assert.Error(t, err)

	err = helper.Symlink("../baz", "foo")
	assert.Error(t, err)

	err = helper.Symlink("../../../foo", "foo/bar/qux")
	assert.Error(t, err)
}

func TestSymlinkInMount(t *testing.T) {
	helper, underlying, source := setup()
	err := helper.Symlink("../baz", "foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.SymlinkArgs)
	assert.Len(t, source.SymlinkArgs, 1)
	assert.Equal(t, source.SymlinkArgs[0],
		[2]string{"../baz", filepath.Join("bar", "qux")})
}

func TestRadlink(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Readlink("bar/qux")
	require.NoError(t, err)

	assert.Len(t, underlying.ReadlinkArgs, 1)
	assert.Equal(t, underlying.ReadlinkArgs[0], filepath.Join("bar", "qux"))
	assert.Empty(t, source.ReadlinkArgs)
}

func TestReadlinkInMount(t *testing.T) {
	helper, underlying, source := setup()
	_, err := helper.Readlink("foo/bar/qux")
	require.NoError(t, err)

	assert.Empty(t, underlying.ReadlinkArgs)
	assert.Len(t, source.ReadlinkArgs, 1)
	assert.Equal(t, source.ReadlinkArgs[0], filepath.Join("bar", "qux"))
}

func TestUnderlyingNotSupported(t *testing.T) {
	h := New(&test.BasicMock{}, "/foo", &test.BasicMock{})
	_, err := h.ReadDir("qux")
	assert.Equal(t, err, billy.ErrNotSupported)
	_, err = h.Readlink("qux")
	assert.Equal(t, err, billy.ErrNotSupported)
}

func TestSourceNotSupported(t *testing.T) {
	_, underlying, _ := setup()
	h := New(underlying, "/foo", &test.BasicMock{})
	_, err := h.ReadDir("foo")
	assert.Equal(t, err, billy.ErrNotSupported)
	_, err = h.Readlink("foo")
	assert.Equal(t, err, billy.ErrNotSupported)
}

func TestCapabilities(t *testing.T) {
	testCapabilities(t, new(test.BasicMock), new(test.BasicMock))
	testCapabilities(t, new(test.BasicMock), new(test.OnlyReadCapFs))
	testCapabilities(t, new(test.BasicMock), new(test.NoLockCapFs))
	testCapabilities(t, new(test.OnlyReadCapFs), new(test.BasicMock))
	testCapabilities(t, new(test.OnlyReadCapFs), new(test.OnlyReadCapFs))
	testCapabilities(t, new(test.OnlyReadCapFs), new(test.NoLockCapFs))
	testCapabilities(t, new(test.NoLockCapFs), new(test.BasicMock))
	testCapabilities(t, new(test.NoLockCapFs), new(test.OnlyReadCapFs))
	testCapabilities(t, new(test.NoLockCapFs), new(test.NoLockCapFs))
}

func testCapabilities(t *testing.T, a, b billy.Basic) {
	aCapabilities := billy.Capabilities(a)
	bCapabilities := billy.Capabilities(b)

	fs := New(a, "/foo", b)
	capabilities := billy.Capabilities(fs)

	unionCapabilities := aCapabilities & bCapabilities

	assert.Equal(t, capabilities, unionCapabilities)

	fs = New(b, "/foo", a)
	capabilities = billy.Capabilities(fs)

	unionCapabilities = aCapabilities & bCapabilities

	assert.Equal(t, capabilities, unionCapabilities)
}
