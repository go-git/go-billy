package test

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type symlinkFS interface {
	Basic
	Symlink
}

func eachSymlinkFS(t *testing.T, test func(t *testing.T, fs symlinkFS)) {
	t.Helper()
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestSymlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "file", nil, 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "link")
		require.NoError(t, err)

		fi, err := fs.Lstat("link")
		require.NoError(t, err)
		assert.Equal(t, "link", fi.Name())

		fi, err = fs.Stat("link")
		require.NoError(t, err)
		assert.Equal(t, "link", fi.Name())
	})
}

func TestSymlinkCrossDirs(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "foo/file", nil, 0o644)
		require.NoError(t, err)

		err = fs.Symlink("../foo/file", "bar/link")
		require.NoError(t, err)

		fi, err := fs.Stat("bar/link")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "link")
	})
}

func TestSymlinkNested(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "file", []byte("hello world!"), 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "linkA")
		require.NoError(t, err)

		err = fs.Symlink("linkA", "linkB")
		require.NoError(t, err)

		fi, err := fs.Stat("linkB")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "linkB")
		assert.Equal(t, fi.Size(), int64(12))
	})
}

func TestSymlinkWithNonExistentdTarget(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := fs.Symlink("file", "link")
		require.NoError(t, err)

		_, err = fs.Stat("link")
		assert.Equal(t, os.IsNotExist(err), true)
	})
}

func TestSymlinkWithExistingLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "link", nil, 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "link")
		assert.ErrorIs(t, err, os.ErrExist)
	})
}

func TestOpenWithSymlinkToRelativePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "dir/file", []byte("foo"), 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "dir/link")
		require.NoError(t, err)

		f, err := fs.Open("dir/link")
		require.NoError(t, err)

		all, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, string(all), "foo")
		assert.Nil(t, f.Close())
	})
}

func TestOpenWithSymlinkToAbsolutePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	if runtime.GOOS == "wasip1" {
		t.Skip("skipping on wasip1")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "dir/file", []byte("foo"), 0o644)
		require.NoError(t, err)

		err = fs.Symlink("/dir/file", "dir/link")
		require.NoError(t, err)

		f, err := fs.Open("dir/link")
		require.NoError(t, err)

		all, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, string(all), "foo")
		assert.Nil(t, f.Close())
	})
}

func TestReadlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "file", nil, 0o644)
		require.NoError(t, err)

		_, err = fs.Readlink("file")
		assert.Error(t, err)
	})
}

func TestReadlinkWithRelativePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "dir/file", nil, 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "dir/link")
		require.NoError(t, err)

		oldname, err := fs.Readlink("dir/link")
		require.NoError(t, err)
		assert.Equal(t, oldname, "file")
	})
}

func TestReadlinkWithAbsolutePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}
	if runtime.GOOS == "wasip1" {
		t.Skip("skipping on wasip1")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "dir/file", nil, 0o644)
		require.NoError(t, err)

		err = fs.Symlink("/dir/file", "dir/link")
		require.NoError(t, err)

		oldname, err := fs.Readlink("dir/link")
		require.NoError(t, err)
		assert.Equal(t, oldname, expectedSymlinkTarget)
	})
}

func TestReadlinkWithNonExistentTarget(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := fs.Symlink("file", "link")
		require.NoError(t, err)

		oldname, err := fs.Readlink("link")
		require.NoError(t, err)
		assert.Equal(t, oldname, "file")
	})
}

func TestReadlinkWithNonExistentLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		_, err := fs.Readlink("link")
		assert.Equal(t, os.IsNotExist(err), true)
	})
}

func TestStatLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	want := customMode
	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)
		require.NoError(t, err)

		err = fs.Symlink("bar", "foo/qux")
		require.NoError(t, err)

		if runtime.GOOS == "windows" {
			want |= 0o022
		}

		fi, err := fs.Stat("foo/qux")
		require.NoError(t, err)
		assert.Equal(t, "qux", fi.Name())
		assert.Equal(t, int64(3), fi.Size())
		assert.Equal(t, want, fi.Mode())
		assert.Equal(t, false, fi.ModTime().IsZero())
		assert.Equal(t, false, fi.IsDir())
	})
}

func TestLstat(t *testing.T) {
	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)
		require.NoError(t, err)

		fi, err := fs.Lstat("foo/bar")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "bar")
		assert.Equal(t, fi.Size(), int64(3))
		assert.Equal(t, fi.Mode()&os.ModeSymlink != 0, false)
		assert.Equal(t, fi.ModTime().IsZero(), false)
		assert.Equal(t, fi.IsDir(), false)
	})
}

func TestLstatLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "foo/bar", []byte("fosddddaaao"), customMode)
		require.NoError(t, err)
		err = fs.Symlink("bar", "foo/qux")
		require.NoError(t, err)

		fi, err := fs.Lstat("foo/qux")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "qux")
		assert.Equal(t, fi.Mode()&os.ModeSymlink != 0, true)
		assert.Equal(t, fi.ModTime().IsZero(), false)
		assert.Equal(t, fi.IsDir(), false)
	})
}

func TestRenameWithSymlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := fs.Symlink("file", "link")
		require.NoError(t, err)

		err = fs.Rename("link", "newlink")
		require.NoError(t, err)

		_, err = fs.Readlink("newlink")
		require.NoError(t, err)
	})
}

func TestRemoveWithSymlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		t.Helper()
		err := util.WriteFile(fs, "file", []byte("foo"), 0o644)
		require.NoError(t, err)

		err = fs.Symlink("file", "link")
		require.NoError(t, err)

		err = fs.Remove("link")
		require.NoError(t, err)

		_, err = fs.Readlink("link")
		assert.Equal(t, os.IsNotExist(err), true)

		_, err = fs.Stat("link")
		assert.Equal(t, os.IsNotExist(err), true)

		_, err = fs.Stat("file")
		require.NoError(t, err)
	})
}
