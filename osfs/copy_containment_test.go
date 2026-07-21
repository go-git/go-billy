//go:build !js

package osfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertDstDoesNotExposeOutsideSecret(t *testing.T, dstDir, name string) {
	t.Helper()
	got, rerr := os.ReadFile(filepath.Join(dstDir, name))
	if os.IsNotExist(rerr) {
		return
	}
	require.NoError(t, rerr)
	assert.NotContains(t, string(got), "outside-secret")
}

func TestCopyFrom_SymlinkEscapeIsContained(t *testing.T) {
	srcDir := t.TempDir()
	src := New(srcDir)

	outsideDir := t.TempDir()
	secret := []byte("outside-secret")
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret"), secret, 0o600))

	require.NoError(t, src.Symlink(filepath.Join(outsideDir, "secret"), "link"))

	dstDir := t.TempDir()
	dst := New(dstDir)

	err := util.Copy(src, "link", dst, "out")
	require.Error(t, err)
	assertDstDoesNotExposeOutsideSecret(t, dstDir, "out")
}

func TestCopyFrom_DotDotTraversalIsContained(t *testing.T) {
	srcDir := t.TempDir()
	src := New(srcDir)

	secret := []byte("outside-secret")
	outsideSecret := filepath.Join(srcDir, "..", "secret")
	require.NoError(t, os.WriteFile(outsideSecret, secret, 0o600))

	dstDir := t.TempDir()
	dst := New(dstDir)

	err := util.Copy(src, filepath.Join("..", "secret"), dst, "out")
	require.Error(t, err)
	assertDstDoesNotExposeOutsideSecret(t, dstDir, "out")
}

func TestCopyFrom_NonOSFSSourceFallsBack(t *testing.T) {
	src := memfs.New()
	require.NoError(t, util.WriteFile(src, "in", []byte("hello"), 0o644))

	dstDir := t.TempDir()
	dst := New(dstDir)

	require.NoError(t, util.Copy(src, "in", dst, "out"))
	got, err := os.ReadFile(filepath.Join(dstDir, "out"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}
