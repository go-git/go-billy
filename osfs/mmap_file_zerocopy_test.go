//go:build darwin || linux

package osfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/require"
)

func TestMmapFileZeroCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := filepath.Join(dir, "data.bin")
	want := []byte("the quick brown fox")
	require.NoError(t, os.WriteFile(name, want, 0o600))

	osRoot, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = osRoot.Close() })

	root, err := FromRoot(osRoot, WithMmap())
	require.NoError(t, err)

	f, err := root.Open("data.bin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	m, ok := f.(billy.Mmap)
	require.True(t, ok, "mmap-backed file must satisfy billy.Mmap")
	require.Equal(t, want, m.Bytes())
	require.Equal(t, []byte("quick"), m.Slice(4, 5))
}
