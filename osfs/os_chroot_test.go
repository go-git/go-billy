//go:build !wasm

package osfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (billy.Filesystem, string) {
	t.Helper()

	path := t.TempDir()
	if runtime.GOOS == "plan9" {
		// On Plan 9, permission mode of newly created files
		// or directories are based on the permission mode of
		// the containing directory (see http://man.cat-v.org/plan_9/5/open).
		// Since TestOpenFileWithModes and TestStat creates files directly
		// in the temporary directory, we need to make it more permissive.
		err := os.Chmod(path, 0o777)
		require.NoError(t, err)
	}
	return newChrootOS(path), path
}

func TestOpenDoesNotCreateDir(t *testing.T) {
	fs, path := setup(t)
	_, err := fs.Open("dir/non-existent")
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(path, "dir"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCapabilities(t *testing.T) {
	fs, _ := setup(t)
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := billy.Capabilities(fs)
	assert.Equal(t, billy.DefaultCapabilities, caps)
}

func TestCreateWithChroot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping POSIX umask tests on Windows")
	}
	fs, _ := setup(t)
	resetUmask := umask(2)
	chroot, _ := fs.Chroot("foo")
	f, err := chroot.Create("bar")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	assert.Equal(t, f.Name(), "bar")
	resetUmask()

	di, err := fs.Stat("foo")
	require.NoError(t, err)
	expected := 0o775
	actual := int(di.Mode().Perm())
	assert.Equal(
		t, expected, actual, "Permission mismatch - expected: 0o%o, actual: 0o%o", expected, actual,
	)
}

// Verify that the Rename() is successful even if the destination is a
// read-only file.
func TestRenameToReadonly(t *testing.T) {
	fs, _ := setup(t)
	chroot, _ := fs.Chroot("rename")

	// Prepare two files: rename source and destination
	err := util.WriteFile(chroot, "src.txt", []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("failed to write src.txt: %s", err)
	}
	err = util.WriteFile(chroot, "dst.txt", []byte("world"), 0o444)
	if err != nil {
		t.Fatalf("failed to write dst.txt: %s", err)
	}

	err = chroot.Rename("src.txt", "dst.txt")
	if err != nil {
		t.Fatalf("failed to rename to overwrite read-only file: %s", err)
	}

	// src.txt must not exist
	_, err = chroot.Stat("src.txt")
	if err == nil {
		t.Error("src.txt must not exist, but does it")
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error on src.txt: %s", err)
	}

	// Check dst.txt's permission and contents.
	fi, err := chroot.Stat("dst.txt")
	if err != nil {
		t.Errorf("unexpected error on dst.txt: %s", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("unexpected permission of dst.txt: %04o", perm)
	}
	b, err := util.ReadFile(chroot, "dst.txt")
	if err != nil {
		t.Errorf("failed to read dst.txt: %s", err)
	} else if string(b) != "hello" {
		t.Errorf("unexpected contents of dst.txt: want=%q got=%q", "hello", string(b))
	}
}
