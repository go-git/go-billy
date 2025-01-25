package test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	. "github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
)

type dirFS interface {
	Basic
	Dir
}

func eachDirFS(t *testing.T, test func(t *testing.T, fs dirFS)) {
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestDir_MkdirAll(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("empty", os.FileMode(0755))
		assert.NoError(t, err)

		fi, err := fs.Stat("empty")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())
	})
}

func TestDir_MkdirAllNested(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("foo/bar/baz", os.FileMode(0755))
		assert.NoError(t, err)

		fi, err := fs.Stat("foo/bar/baz")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())

		fi, err = fs.Stat("foo/bar")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())

		fi, err = fs.Stat("foo")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())
	})
}

func TestDir_MkdirAllIdempotent(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("empty", 0755)
		assert.NoError(t, err)
		fi, err := fs.Stat("empty")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())

		// idempotent
		err = fs.MkdirAll("empty", 0755)
		assert.NoError(t, err)
		fi, err = fs.Stat("empty")
		assert.NoError(t, err)
		assert.True(t, fi.IsDir())
	})
}

func TestDir_MkdirAllAndCreate(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("dir", os.FileMode(0755))
		assert.NoError(t, err)

		f, err := fs.Create("dir/bar/foo")
		assert.NoError(t, err)
		assert.Nil(t, f.Close())

		fi, err := fs.Stat("dir/bar/foo")
		assert.NoError(t, err)
		assert.False(t, fi.IsDir())
	})
}

func TestDir_MkdirAllWithExistingFile(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		f, err := fs.Create("dir/foo")
		assert.NoError(t, err)
		assert.Nil(t, f.Close())

		err = fs.MkdirAll("dir/foo", os.FileMode(0755))
		assert.Error(t, err)

		fi, err := fs.Stat("dir/foo")
		assert.NoError(t, err)
		assert.False(t, fi.IsDir())
	})
}

func TestDir_StatDir(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		fs.MkdirAll("foo/bar", 0755)

		fi, err := fs.Stat("foo/bar")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "bar")
		assert.Equal(t, fi.Mode().IsDir(), true)
		assert.Equal(t, fi.ModTime().IsZero(), false)
		assert.True(t, fi.IsDir())
	})
}

func TestDir_StatDeep(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			assert.NoError(t, err)
		}

		// Some implementations detect directories based on a prefix
		// for all files; it's easy to miss path separator handling there.
		fi, err := fs.Stat("qu")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, fi)

		fi, err = fs.Stat("qux")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "qux")
		assert.True(t, fi.IsDir())

		fi, err = fs.Stat("qux/baz")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "baz")
		assert.False(t, fi.IsDir())
	})
}

func TestDir_ReadDir(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			assert.NoError(t, err)
		}

		info, err := fs.ReadDir("/")
		assert.NoError(t, err)
		assert.Len(t, info, 3)

		info, err = fs.ReadDir("/qux")
		assert.NoError(t, err)
		assert.Len(t, info, 2)
	})
}

func TestDir_ReadDirNested(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		max := 100
		path := "/"
		for i := 0; i <= max; i++ {
			path = fs.Join(path, strconv.Itoa(i))
		}

		files := []string{fs.Join(path, "f1"), fs.Join(path, "f2")}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			assert.NoError(t, err)
		}

		path = "/"
		for i := 0; i < max; i++ {
			path = fs.Join(path, strconv.Itoa(i))
			info, err := fs.ReadDir(path)
			assert.NoError(t, err)
			assert.Len(t, info, 1)
		}

		path = fs.Join(path, strconv.Itoa(max))
		info, err := fs.ReadDir(path)
		assert.NoError(t, err)
		assert.Len(t, info, 2)
	})
}

func TestDir_ReadDirWithMkDirAll(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("qux", 0755)
		assert.NoError(t, err)

		files := []string{"qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			assert.NoError(t, err)
		}

		info, err := fs.ReadDir("/")
		assert.NoError(t, err)
		assert.Len(t, info, 1)
		assert.Equal(t, info[0].IsDir(), true)

		info, err = fs.ReadDir("/qux")
		assert.NoError(t, err)
		assert.Len(t, info, 2)
	})
}

func TestDir_ReadDirFileInfo(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := util.WriteFile(fs, "foo", []byte{'F', 'O', 'O'}, 0644)
		assert.NoError(t, err)

		info, err := fs.ReadDir("/")
		assert.NoError(t, err)
		assert.Len(t, info, 1)

		assert.Equal(t, info[0].Size(), int64(3))
		assert.Equal(t, info[0].IsDir(), false)
		assert.Equal(t, info[0].Name(), "foo")
	})
}

func TestDir_ReadDirFileInfoDirs(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		files := []string{"qux/baz/foo"}
		for _, name := range files {
			err := util.WriteFile(fs, name, []byte{'F', 'O', 'O'}, 0644)
			assert.NoError(t, err)
		}

		info, err := fs.ReadDir("qux")
		assert.NoError(t, err)
		assert.Len(t, info, 1)
		assert.Equal(t, info[0].IsDir(), true)
		assert.Equal(t, info[0].Name(), "baz")

		info, err = fs.ReadDir("qux/baz")
		assert.NoError(t, err)
		assert.Len(t, info, 1)
		assert.Equal(t, info[0].Size(), int64(3))
		assert.Equal(t, info[0].IsDir(), false)
		assert.Equal(t, info[0].Name(), "foo")
		assert.NotEqual(t, info[0].Mode(), 0)
	})
}

func TestDir_RenameToDir(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := util.WriteFile(fs, "foo", nil, 0644)
		assert.NoError(t, err)

		err = fs.Rename("foo", "bar/qux")
		assert.NoError(t, err)

		old, err := fs.Stat("foo")
		assert.Nil(t, old)
		assert.Equal(t, os.IsNotExist(err), true)

		dir, err := fs.Stat("bar")
		assert.NotNil(t, dir)
		assert.NoError(t, err)

		file, err := fs.Stat("bar/qux")
		assert.Equal(t, file.Name(), "qux")
		assert.NoError(t, err)
	})
}

func TestDir_RenameDir(t *testing.T) {
	eachDirFS(t, func(t *testing.T, fs dirFS) {
		err := fs.MkdirAll("foo", 0755)
		assert.NoError(t, err)

		err = util.WriteFile(fs, "foo/bar", nil, 0644)
		assert.NoError(t, err)

		err = fs.Rename("foo", "bar")
		assert.NoError(t, err)

		dirfoo, err := fs.Stat("foo")
		assert.Nil(t, dirfoo)
		assert.Equal(t, os.IsNotExist(err), true)

		dirbar, err := fs.Stat("bar")
		assert.NoError(t, err)
		assert.NotNil(t, dirbar)

		foo, err := fs.Stat("foo/bar")
		assert.Equal(t, os.IsNotExist(err), true)
		assert.Nil(t, foo)

		bar, err := fs.Stat("bar/bar")
		assert.NoError(t, err)
		assert.NotNil(t, bar)
	})
}
