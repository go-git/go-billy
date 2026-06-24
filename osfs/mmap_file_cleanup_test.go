//go:build linux

package osfs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootOSMmapCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	_ = writePattern(t, path, 64*1024)

	fs := newTestBoundOS(t, dir, WithMmap())
	f, err := fs.Open("data")
	require.NoError(t, err)

	ok, err := isFileMapped(path)
	require.NoError(t, err)
	require.True(t, ok)

	_ = f.Name()
	f = nil
	_ = f

	for range 10 {
		runtime.GC()

		ok, err = isFileMapped(path)
		require.NoError(t, err)
		if !ok {
			break
		}
	}

	ok, err = isFileMapped(path)
	require.NoError(t, err)
	require.False(t, ok)
}

// isFileMapped checks if a path appears in the proc maps file. Only tested
// in Linux.
func isFileMapped(path string) (bool, error) {
	pid := os.Getpid()

	mapsPath := fmt.Sprintf("/proc/%d/maps", pid)
	f, err := os.Open(mapsPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()
		if strings.Contains(l, path) {
			return true, nil
		}
	}

	return false, s.Err()
}
