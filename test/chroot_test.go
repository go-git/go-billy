package test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/osfs"
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
		err := util.WriteFile(fs, "bar", []byte("root"), 0o644)
		require.NoError(t, err)
		err = util.WriteFile(fs, "foo/bar", []byte("chroot"), 0o644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Open("../bar")
		if _, ok := chroot.(*osfs.BoundOS); !ok {
			require.ErrorIs(t, err, ErrCrossedBoundary)
			assert.Nil(t, f)
			assertFileContents(t, fs, "bar", []byte("root"))
			assertFileContents(t, fs, "foo/bar", []byte("chroot"))
			return
		}

		require.NoError(t, err)
		require.NoError(t, f.Close())
		data, err := util.ReadFile(chroot, "../bar")
		require.NoError(t, err)
		assert.Equal(t, []byte("chroot"), data)
		assertFileContents(t, fs, "bar", []byte("root"))
	})
}

func TestStatOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", []byte("root"), 0o644)
		require.NoError(t, err)
		err = util.WriteFile(fs, "foo/bar", []byte("chroot"), 0o644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Stat("../bar")
		if _, ok := chroot.(*osfs.BoundOS); !ok {
			require.ErrorIs(t, err, ErrCrossedBoundary)
			assert.Nil(t, f)
			assertFileContents(t, fs, "bar", []byte("root"))
			assertFileContents(t, fs, "foo/bar", []byte("chroot"))
			return
		}

		require.NoError(t, err)
		assert.Equal(t, int64(len("chroot")), f.Size())
		assertFileContents(t, fs, "bar", []byte("root"))
	})
}

func TestStatWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0o644)
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
		err := util.WriteFile(fs, "bar", []byte("root"), 0o644)
		require.NoError(t, err)
		err = util.WriteFile(fs, "foo/bar", []byte("chroot-from"), 0o644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Rename("../bar", "renamed")
		if _, ok := chroot.(*osfs.BoundOS); !ok {
			require.ErrorIs(t, err, ErrCrossedBoundary)
			assertFileContents(t, fs, "bar", []byte("root"))
			assertFileContents(t, fs, "foo/bar", []byte("chroot-from"))
			return
		}

		require.NoError(t, err)
		assertFileContents(t, fs, "bar", []byte("root"))
		assertFileContents(t, fs, "foo/renamed", []byte("chroot-from"))
	})
}

func TestRenameIntoParentBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", []byte("root"), 0o644)
		require.NoError(t, err)
		err = util.WriteFile(fs, "foo/file", []byte("chroot-to"), 0o644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Rename("file", "../bar")
		if _, ok := chroot.(*osfs.BoundOS); ok {
			require.NoError(t, err)
			assertFileContents(t, fs, "bar", []byte("root"))
			assertFileContents(t, fs, "foo/bar", []byte("chroot-to"))
			return
		}

		require.ErrorIs(t, err, ErrCrossedBoundary)
		assertFileContents(t, fs, "bar", []byte("root"))
		assertFileContents(t, fs, "foo/file", []byte("chroot-to"))
	})
}

func TestRemoveOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		err := util.WriteFile(fs, "bar", []byte("root"), 0o644)
		require.NoError(t, err)
		err = util.WriteFile(fs, "foo/bar", []byte("chroot"), 0o644)
		require.NoError(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Remove("../bar")
		if _, ok := chroot.(*osfs.BoundOS); ok {
			require.NoError(t, err)
			_, err = fs.Open("foo/bar")
			assert.ErrorIs(t, err, os.ErrNotExist)
		} else {
			require.ErrorIs(t, err, ErrCrossedBoundary)
			assertFileContents(t, fs, "foo/bar", []byte("chroot"))
		}
		assertFileContents(t, fs, "bar", []byte("root"))
	})
}

func assertFileContents(t *testing.T, fs Basic, path string, want []byte) {
	t.Helper()
	data, err := util.ReadFile(fs, path)
	require.NoError(t, err)
	assert.Equal(t, want, data)
}

func TestRoot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		t.Helper()
		assert.NotEmpty(t, fs.Root())
	})
}
