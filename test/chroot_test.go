package test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"
)

type chrootFS interface {
	Basic
	Chroot
}

func eachChrootFS(t *testing.T, test func(t *testing.T, fs chrootFS)) {
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestCreateWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Create("bar")
		assert.Nil(t, err)
		assert.Nil(t, f.Close())
		assert.Equal(t, f.Name(), "bar")

		f, err = fs.Open("foo/bar")
		assert.Nil(t, err)
		assert.Equal(t, f.Name(), fs.Join("foo", "bar"))
		assert.Nil(t, f.Close())
	})
}

func TestOpenWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Create("bar")
		assert.Nil(t, err)
		assert.Nil(t, f.Close())
		assert.Equal(t, f.Name(), "bar")

		f, err = chroot.Open("bar")
		assert.Nil(t, err)
		assert.Equal(t, f.Name(), "bar")
		assert.Nil(t, f.Close())
	})
}

func TestOpenOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		err := util.WriteFile(fs, "bar", nil, 0644)
		assert.Nil(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Open("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
		assert.Nil(t, f)
	})
}

func TestStatOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		err := util.WriteFile(fs, "bar", nil, 0644)
		assert.Nil(t, err)

		chroot, _ := fs.Chroot("foo")
		f, err := chroot.Stat("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
		assert.Nil(t, f)
	})
}

func TestStatWithChroot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		files := []string{"foo", "bar", "qux/baz", "qux/qux"}
		for _, name := range files {
			err := util.WriteFile(fs, name, nil, 0644)
			assert.Nil(t, err)
		}

		// Some implementations detect directories based on a prefix
		// for all files; it's easy to miss path separator handling there.
		fi, err := fs.Stat("qu")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, fi)

		fi, err = fs.Stat("qux")
		assert.Nil(t, err)
		assert.Equal(t, fi.Name(), "qux")
		assert.Equal(t, fi.IsDir(), true)

		qux, _ := fs.Chroot("qux")

		fi, err = qux.Stat("baz")
		assert.Nil(t, err)
		assert.Equal(t, fi.Name(), "baz")
		assert.Equal(t, fi.IsDir(), false)

		fi, err = qux.Stat("/baz")
		assert.Nil(t, err)
		assert.Equal(t, fi.Name(), "baz")
		assert.Equal(t, fi.IsDir(), false)
	})
}

func TestRenameOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		err := util.WriteFile(fs, "foo/foo", nil, 0644)
		assert.Nil(t, err)

		err = util.WriteFile(fs, "bar", nil, 0644)
		assert.Nil(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Rename("../bar", "foo")
		assert.Equal(t, err, ErrCrossedBoundary)

		err = chroot.Rename("foo", "../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
	})
}

func TestRemoveOutOffBoundary(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		err := util.WriteFile(fs, "bar", nil, 0644)
		assert.Nil(t, err)

		chroot, _ := fs.Chroot("foo")
		err = chroot.Remove("../bar")
		assert.Equal(t, err, ErrCrossedBoundary)
	})
}

func TestRoot(t *testing.T) {
	eachChrootFS(t, func(t *testing.T, fs chrootFS) {
		assert.NotEmpty(t, fs.Root())
	})
}
