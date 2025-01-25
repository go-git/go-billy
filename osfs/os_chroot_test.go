//go:build !wasm
// +build !wasm

package osfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) (billy.Filesystem, string) {
	path := t.TempDir()
	if runtime.GOOS == "plan9" {
		// On Plan 9, permission mode of newly created files
		// or directories are based on the permission mode of
		// the containing directory (see http://man.cat-v.org/plan_9/5/open).
		// Since TestOpenFileWithModes and TestStat creates files directly
		// in the temporary directory, we need to make it more permissive.
		err := os.Chmod(path, 0777)
		assert.NoError(t, err)
	}
	return newChrootOS(path), path
}

func TestOpenDoesNotCreateDir(t *testing.T) {
	fs, path := setup(t)
	_, err := fs.Open("dir/non-existent")
	assert.Error(t, err)

	_, err = os.Stat(filepath.Join(path, "dir"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCapabilities(t *testing.T) {
	fs, _ := setup(t)
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := billy.Capabilities(fs)
	assert.Equal(t, billy.AllCapabilities, caps)
}
