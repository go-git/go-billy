//go:build darwin || linux

package osfs

import (
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/require"
)

func assertMmapBackingWhenAvailable(t *testing.T, f billy.File) {
	t.Helper()
	_, ok := f.(*mmapFile)
	require.True(t, ok, "WithMmap should select *mmapFile on this platform, got %T", f)
}

// Ensure *mmapFile satisfies billy.File at compile-time on platforms
// where it has a real implementation.
var _ billy.File = (*mmapFile)(nil)
