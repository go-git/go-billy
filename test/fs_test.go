package test

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func eachFS(t *testing.T, test func(t *testing.T, fs Filesystem)) {
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestFS_SymlinkToDir(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	eachFS(t, func(t *testing.T, fs Filesystem) {
		err := fs.MkdirAll("dir", 0755)
		require.NoError(t, err)

		err = fs.Symlink("dir", "link")
		require.NoError(t, err)

		fi, err := fs.Stat("link")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "link")
		assert.True(t, fi.IsDir())
	})
}

func TestFS_SymlinkReadDir(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	eachFS(t, func(t *testing.T, fs Filesystem) {
		err := util.WriteFile(fs, "dir/file", []byte("foo"), 0644)
		require.NoError(t, err)

		err = fs.Symlink("dir", "link")
		require.NoError(t, err)

		info, err := fs.ReadDir("link")
		require.NoError(t, err)
		assert.Len(t, info, 1)

		assert.Equal(t, info[0].Size(), int64(3))
		assert.Equal(t, info[0].IsDir(), false)
		assert.Equal(t, info[0].Name(), "file")
	})
}

func TestFS_CreateWithExistantDir(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		err := fs.MkdirAll("foo", 0644)
		require.NoError(t, err)

		f, err := fs.Create("foo")
		assert.Error(t, err)
		assert.Nil(t, f)
	})
}

func TestFS_ReadDirWithChroot(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			require.NoError(t, err)
		}

		qux, _ := fs.Chroot("/qux")

		info, err := qux.ReadDir("/")
		require.NoError(t, err)
		assert.Len(t, info, 2)
	})
}

func TestFS_SymlinkWithChrootBasic(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	eachFS(t, func(t *testing.T, fs Filesystem) {
		qux, _ := fs.Chroot("/qux")

		err := util.WriteFile(qux, "file", nil, 0644)
		require.NoError(t, err)

		err = qux.Symlink("file", "link")
		require.NoError(t, err)

		fi, err := qux.Stat("link")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "link")

		fi, err = fs.Stat("qux/link")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "link")
	})
}

func TestFS_SymlinkWithChrootCrossBounders(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	eachFS(t, func(t *testing.T, fs Filesystem) {
		qux, _ := fs.Chroot("/qux")
		err := util.WriteFile(fs, "file", []byte("foo"), customMode)
		require.NoError(t, err)

		err = qux.Symlink("../../file", "qux/link")
		assert.Equal(t, err, nil)

		fi, err := qux.Stat("qux/link")
		assert.NotNil(t, fi)
		assert.Equal(t, err, nil)
	})
}

func TestFS_ReadDirWithLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	eachFS(t, func(t *testing.T, fs Filesystem) {
		err := util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)
		require.NoError(t, err)

		err = fs.Symlink("bar", "foo/qux")
		require.NoError(t, err)

		info, err := fs.ReadDir("/foo")
		require.NoError(t, err)
		assert.Len(t, info, 2)
	})
}

func TestFS_RemoveAllNonExistent(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		require.NoError(t, util.RemoveAll(fs, "non-existent"))
	})
}

func TestFS_RemoveAllEmptyDir(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		require.NoError(t, fs.MkdirAll("empty", os.FileMode(0755)))
		require.NoError(t, util.RemoveAll(fs, "empty"))
		_, err := fs.Stat("empty")
		assert.Error(t, err)
		assert.Equal(t, os.IsNotExist(err), true)
	})
}

func TestFS_RemoveAll(t *testing.T) {
	fnames := []string{
		"foo/1",
		"foo/2",
		"foo/bar/1",
		"foo/bar/2",
		"foo/bar/baz/1",
		"foo/bar/baz/qux/1",
		"foo/bar/baz/qux/2",
		"foo/bar/baz/qux/3",
	}

	eachFS(t, func(t *testing.T, fs Filesystem) {
		for _, fname := range fnames {
			err := util.WriteFile(fs, fname, nil, 0644)
			require.NoError(t, err)
		}

		require.NoError(t, util.RemoveAll(fs, "foo"))

		for _, fname := range fnames {
			_, err := fs.Stat(fname)
			assert.ErrorIsf(t, err, os.ErrNotExist, "not removed: %s %s", fname, err)
		}
	})
}

func TestFS_RemoveAllRelative(t *testing.T) {
	fnames := []string{
		"foo/1",
		"foo/2",
		"foo/bar/1",
		"foo/bar/2",
		"foo/bar/baz/1",
		"foo/bar/baz/qux/1",
		"foo/bar/baz/qux/2",
		"foo/bar/baz/qux/3",
	}

	eachFS(t, func(t *testing.T, fs Filesystem) {
		for _, fname := range fnames {
			err := util.WriteFile(fs, fname, nil, 0644)
			require.NoError(t, err)
		}

		require.NoError(t, util.RemoveAll(fs, "foo/bar/.."))

		for _, fname := range fnames {
			_, err := fs.Stat(fname)
			assert.ErrorIsf(t, err, os.ErrNotExist, "not removed: %s %s", fname, err)
		}
	})
}

func TestFS_ReadDir(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		err := fs.MkdirAll("qux", 0755)
		require.NoError(t, err)

		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			require.NoError(t, err)
		}

		info, err := fs.ReadDir("/")
		require.NoError(t, err)
		assert.Len(t, info, 3)

		info, err = fs.ReadDir("/qux")
		require.NoError(t, err)
		assert.Len(t, info, 2)
	})
}
