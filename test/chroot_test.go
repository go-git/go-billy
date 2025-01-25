package test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chrootFS interface {
	Basic
	Chroot
}

func eachChrootFS(t *testing.T, test func(t *testing.T, fs chrootFS)) {
	t.Helper()
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestCreateWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Create("bar")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		assert.Equal(t, f.Name(), "bar")

		f, err = fs.Open("foo/bar")
		require.NoError(t, err)
		assert.Equal(t, f.Name(), fs.Join("foo", "bar"))
		require.NoError(t, f.Close())
	})
}

func TestOpenWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Create("bar")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		assert.Equal(t, f.Name(), "bar")

		f, err = chroot.Open("bar")
		require.NoError(t, err)
		assert.Equal(t, f.Name(), "bar")
		require.NoError(t, f.Close())
	})
}

func TestOpenOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", nil, 0644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Open("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
		assert.Nil(t, f)
	})
}

func TestStatOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", nil, 0644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Stat("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
		assert.Nil(t, f)
	})
}

func TestStatWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			require.NoError(t, err)
		}

		// Some implementations detect directories based on a prefix
		// for all files; it's easy to miss path separator handling there.
		fi, err := fs.Stat("qu")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, fi)

		fi, err = fs.Stat("qux")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "qux")
		assert.True(t, fi.IsDir())

		qux, _ := fs.Chroot("qux")

		fi, err = qux.Stat("baz")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "baz")
		assert.False(t, fi.IsDir())

		fi, err = qux.Stat("/baz")
		require.NoError(t, err)
		assert.Equal(t, fi.Name(), "baz")
		assert.False(t, fi.IsDir())
	})
}

func TestRenameOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "foo/foo", nil, 0644)
		require.NoError(t, err)

		err = util.WriteFile(fs, "bar", nil, 0644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Rename("../bar", "foo")
		assert.Equal(t, err, ErrCrossedBoundary)

		err = chroot.Rename("foo", "../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
	})
}

func TestRemoveOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", nil, 0644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Remove("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
	})
}

func TestRoot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		assert.NotEmpty(t, fs.Root())
	})
}
