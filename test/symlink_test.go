package test

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	. "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"
)

type symlinkFS interface {
	Basic
	Symlink
}

func eachSymlinkFS(t *testing.T, test func(t *testing.T, fs symlinkFS)) {
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
		err := util.WriteFile(fs, "file", nil, 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "link")
		assert.NoError(t, err)

		fi, err := fs.Lstat("link")
		assert.NoError(t, err)
		assert.Equal(t, "link", fi.Name())

		fi, err = fs.Stat("link")
		assert.NoError(t, err)
		assert.Equal(t, "link", fi.Name())
	})
}

func TestSymlinkCrossDirs(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "foo/file", nil, 0644)
		assert.NoError(t, err)

		err = fs.Symlink("../foo/file", "bar/link")
		assert.NoError(t, err)

		fi, err := fs.Stat("bar/link")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "link")
	})
}

func TestSymlinkNested(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "file", []byte("hello world!"), 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "linkA")
		assert.NoError(t, err)

		err = fs.Symlink("linkA", "linkB")
		assert.NoError(t, err)

		fi, err := fs.Stat("linkB")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "linkB")
		assert.Equal(t, fi.Size(), int64(12))
	})
}

func TestSymlinkWithNonExistentdTarget(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := fs.Symlink("file", "link")
		assert.NoError(t, err)

		_, err = fs.Stat("link")
		assert.Equal(t, os.IsNotExist(err), true)
	})
}

func TestSymlinkWithExistingLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "link", nil, 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "link")
		assert.ErrorIs(t, err, os.ErrExist)
	})
}

func TestOpenWithSymlinkToRelativePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "dir/file", []byte("foo"), 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "dir/link")
		assert.NoError(t, err)

		f, err := fs.Open("dir/link")
		assert.NoError(t, err)

		all, err := io.ReadAll(f)
		assert.NoError(t, err)
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
		err := util.WriteFile(fs, "dir/file", []byte("foo"), 0644)
		assert.NoError(t, err)

		err = fs.Symlink("/dir/file", "dir/link")
		assert.NoError(t, err)

		f, err := fs.Open("dir/link")
		assert.NoError(t, err)

		all, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(all), "foo")
		assert.Nil(t, f.Close())
	})
}

func TestReadlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "file", nil, 0644)
		assert.NoError(t, err)

		_, err = fs.Readlink("file")
		assert.Error(t, err)
	})
}

func TestReadlinkWithRelativePath(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "dir/file", nil, 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "dir/link")
		assert.NoError(t, err)

		oldname, err := fs.Readlink("dir/link")
		assert.NoError(t, err)
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
		err := util.WriteFile(fs, "dir/file", nil, 0644)
		assert.NoError(t, err)

		err = fs.Symlink("/dir/file", "dir/link")
		assert.NoError(t, err)

		oldname, err := fs.Readlink("dir/link")
		assert.NoError(t, err)
		assert.Equal(t, oldname, expectedSymlinkTarget)
	})
}

func TestReadlinkWithNonExistentTarget(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := fs.Symlink("file", "link")
		assert.NoError(t, err)

		oldname, err := fs.Readlink("link")
		assert.NoError(t, err)
		assert.Equal(t, oldname, "file")
	})
}

func TestReadlinkWithNonExistentLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		_, err := fs.Readlink("link")
		assert.Equal(t, os.IsNotExist(err), true)
	})
}

func TestStatLink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)
		fs.Symlink("bar", "foo/qux")

		fi, err := fs.Stat("foo/qux")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "qux")
		assert.Equal(t, fi.Size(), int64(3))
		assert.Equal(t, fi.Mode(), customMode)
		assert.Equal(t, fi.ModTime().IsZero(), false)
		assert.Equal(t, fi.IsDir(), false)
	})
}

func TestLstat(t *testing.T) {
	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)

		fi, err := fs.Lstat("foo/bar")
		assert.NoError(t, err)
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
		util.WriteFile(fs, "foo/bar", []byte("fosddddaaao"), customMode)
		fs.Symlink("bar", "foo/qux")

		fi, err := fs.Lstat("foo/qux")
		assert.NoError(t, err)
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
		err := fs.Symlink("file", "link")
		assert.NoError(t, err)

		err = fs.Rename("link", "newlink")
		assert.NoError(t, err)

		_, err = fs.Readlink("newlink")
		assert.NoError(t, err)
	})
}

func TestRemoveWithSymlink(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on Plan 9; symlinks are not supported")
	}

	eachSymlinkFS(t, func(t *testing.T, fs symlinkFS) {
		err := util.WriteFile(fs, "file", []byte("foo"), 0644)
		assert.NoError(t, err)

		err = fs.Symlink("file", "link")
		assert.NoError(t, err)

		err = fs.Remove("link")
		assert.NoError(t, err)

		_, err = fs.Readlink("link")
		assert.Equal(t, os.IsNotExist(err), true)

		_, err = fs.Stat("link")
		assert.Equal(t, os.IsNotExist(err), true)

		_, err = fs.Stat("file")
		assert.NoError(t, err)
	})
}
