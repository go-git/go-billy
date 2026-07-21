//go:build linux

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

func TestCopyFrom_CopyFileRange(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "dst"), 0o755))
	src := osfs.New(filepath.Join(tmp, "src"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "dst"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteLinux(src, "f", []byte("copy me"), 0o644))

	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	got, err := os.ReadFile(filepath.Join(tmp, "dst", "f"))
	require.NoError(t, err)
	assert.Equal(t, "copy me", string(got))
}

func TestCopyFrom_LinuxFallbackPreservesMode(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "s"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "d"), 0o755))
	src := osfs.New(filepath.Join(tmp, "s"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "d"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteLinux(src, "f", []byte("x"), 0o600))
	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	fi, err := os.Stat(filepath.Join(tmp, "d", "f"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}

func TestCopyFrom_LinuxEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "s"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "d"), 0o755))
	src := osfs.New(filepath.Join(tmp, "s"), osfs.WithBoundOS())
	dst := osfs.New(filepath.Join(tmp, "d"), osfs.WithBoundOS())
	require.NoError(t, billyCopyFileWriteLinux(src, "f", []byte{}, 0o644))
	copier, ok := dst.(billy.Copier)
	require.True(t, ok)
	require.NoError(t, copier.CopyFrom(src, "f", "f"))
	got, err := os.ReadFile(filepath.Join(tmp, "d", "f"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func billyCopyFileWriteLinux(fs billy.Filesystem, name string, data []byte, perm os.FileMode) error {
	f, err := fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
