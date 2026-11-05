//go:build windows

package osfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFrom_BasicCopy(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "dst"), 0o755))
	src := osfs.New(filepath.Join(tmp, "src"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "dst"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteWindows(src, "f", []byte("copy me"), 0o644))

	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	got, err := os.ReadFile(filepath.Join(tmp, "dst", "f"))
	require.NoError(t, err)
	assert.Equal(t, "copy me", string(got))
}

func TestCopyFrom_WindowsOverwrite(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "s"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "d"), 0o755))
	src := osfs.New(filepath.Join(tmp, "s"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "d"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteWindows(src, "f", []byte("new"), 0o644))
	require.NoError(t, billyCopyFileWriteWindows(dst, "f", []byte("old-contents-to-truncate"), 0o644))

	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	got, err := os.ReadFile(filepath.Join(tmp, "d", "f"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

func TestCopyFrom_WindowsEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "s"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "d"), 0o755))
	src := osfs.New(filepath.Join(tmp, "s"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "d"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteWindows(src, "f", []byte{}, 0o644))
	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	got, err := os.ReadFile(filepath.Join(tmp, "d", "f"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func billyCopyFileWriteWindows(fs billy.Filesystem, name string, data []byte, perm os.FileMode) error {
	f, err := fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
