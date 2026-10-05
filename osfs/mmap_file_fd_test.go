//go:build linux

package osfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/require"
)

func countOpenFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	return len(entries)
}

// TestMmapFileReleasesFD must not call t.Parallel: it measures
// process-wide descriptors, and top-level non-parallel tests run before
// any parallel test resumes.
func TestMmapFileReleasesFD(t *testing.T) {
	dir := t.TempDir()
	const n = 16
	for i := range n {
		writePattern(t, filepath.Join(dir, fmt.Sprintf("f%d", i)), 4096)
	}
	fs := newTestBoundOS(t, dir, WithMmap())

	// Collect handles leaked by earlier tests so their finalizers or
	// cleanups cannot close descriptors between the two counts.
	runtime.GC()
	runtime.GC()
	before := countOpenFDs(t)
	files := make([]billy.File, 0, n)
	for i := range n {
		f, err := fs.Open(fmt.Sprintf("f%d", i))
		require.NoError(t, err)
		assertMmapBackingWhenAvailable(t, f)
		files = append(files, f)
	}
	after := countOpenFDs(t)
	for _, f := range files {
		require.NoError(t, f.Close())
	}

	require.Equal(t, before, after, "mmap-backed files must not hold descriptors")
}

func TestMmapFileStatAfterUnlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	writePattern(t, path, 4096)

	f, err := newTestBoundOS(t, dir, WithMmap()).Open("data")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	assertMmapBackingWhenAvailable(t, f)

	require.NoError(t, os.Remove(path))

	info, err := f.Stat()
	require.NoError(t, err)
	require.Equal(t, int64(4096), info.Size())
	require.Equal(t, "data", info.Name())

	buf := make([]byte, 16)
	_, err = f.ReadAt(buf, 0)
	require.NoError(t, err)
}

func TestMmapFileStatAfterClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePattern(t, filepath.Join(dir, "data"), 4096)

	f, err := newTestBoundOS(t, dir, WithMmap()).Open("data")
	require.NoError(t, err)
	assertMmapBackingWhenAvailable(t, f)
	require.NoError(t, f.Close())

	_, err = f.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
}

// TestMmapProcfsFallsBackToFD guards the size-0 fallback: procfs files
// report as regular files of size 0 yet have content, so they must keep
// the fd-backed wrapper under WithMmap.
func TestMmapProcfsFallsBackToFD(t *testing.T) {
	t.Parallel()

	fs := newTestBoundOS(t, fmt.Sprintf("/proc/%d", os.Getpid()), WithMmap())
	f, err := fs.Open("status")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	_, ok := f.(*file)
	require.True(t, ok, "size-0 procfs file must use the fd backing, got %T", f)

	b, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Contains(t, string(b), "Name:")
}
