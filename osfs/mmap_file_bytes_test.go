//go:build darwin || linux

package osfs

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMmapFileBytes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := writePattern(t, filepath.Join(dir, "data"), 64*1024)

	f, err := newTestBoundOS(t, dir, WithMmap()).Open("data")
	require.NoError(t, err)

	bs, ok := f.(billy.BytesFile)
	require.True(t, ok, "mmap-backed file should implement Bytes, got %T", f)

	got, ok := bs.Bytes()
	require.True(t, ok)
	require.Equal(t, want, got)

	require.NoError(t, f.Close())

	got, ok = bs.Bytes()
	require.False(t, ok)
	require.Nil(t, got)
}

func TestMmapFileBytesFromRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := writePattern(t, filepath.Join(dir, "data"), 4096)

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	fs, err := FromRoot(root, WithMmap())
	require.NoError(t, err)

	f, err := fs.Open("data")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	bs, ok := f.(billy.BytesFile)
	require.True(t, ok, "mmap-backed file should implement Bytes, got %T", f)
	got, ok := bs.Bytes()
	require.True(t, ok)
	require.Equal(t, want, got)
}

// TestMmapFileBytesConcurrentClose runs Bytes against Close under -race.
// It only inspects len of the returned slice, never its contents, since
// touching the memory after Close would fault.
func TestMmapFileBytesConcurrentClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePattern(t, filepath.Join(dir, "data"), 4096)

	f, err := newTestBoundOS(t, dir, WithMmap()).Open("data")
	require.NoError(t, err)
	bs, ok := f.(billy.BytesFile)
	require.True(t, ok)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if b, ok := bs.Bytes(); ok {
					assert.Len(t, b, 4096)
				}
			}
		})
	}
	require.NoError(t, f.Close())
	wg.Wait()

	data, ok := bs.Bytes()
	require.False(t, ok)
	require.Nil(t, data)
}

// TestMmapFileBytesThroughRootOSChroot exercises the chroot helper's
// forwarding: RootOS.Chroot wraps every opened file in chroot's file
// type, which would otherwise hide Bytes.
func TestMmapFileBytesThroughRootOSChroot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
	want := writePattern(t, filepath.Join(dir, "sub", "data"), 4096)

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	fs, err := FromRoot(root, WithMmap())
	require.NoError(t, err)
	sub, err := fs.Chroot("sub")
	require.NoError(t, err)

	f, err := sub.Open("data")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	bs, ok := f.(billy.BytesFile)
	require.True(t, ok, "chroot-wrapped mmap file should implement Bytes, got %T", f)
	got, ok := bs.Bytes()
	require.True(t, ok)
	require.Equal(t, want, got)
}
