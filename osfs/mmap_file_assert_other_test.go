//go:build !darwin && !linux && !wasm

package osfs

import (
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/require"
)

func assertMmapBackingWhenAvailable(t *testing.T, f billy.File) {
	t.Helper()
	// mmap is unavailable on this platform; WithMmap is honoured as
	// best-effort and falls through to the fd-backed wrapper.
	_, ok := f.(*file)
	require.True(t, ok, "platform has no mmap path, expected *file, got %T", f)
}
