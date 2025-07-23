package embedfs

import (
	"os"
	"testing"

	"github.com/go-git/go-billy/v6/embedfs_testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Directory operation tests adapted from test/dir_test.go for read-only embedfs

func TestDir_ReadDirNested(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test reading nested directories
	entries, err := fs.ReadDir("/testdata/subdir")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "nested.txt", entries[0].Name())
	assert.False(t, entries[0].IsDir())
}

func TestDir_ReadDirFileInfo(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	entries, err := fs.ReadDir("/testdata")
	require.NoError(t, err)
	
	// Verify all entries have proper FileInfo
	for _, entry := range entries {
		assert.NotEmpty(t, entry.Name())
		assert.NotNil(t, entry.ModTime())
		assert.Greater(t, entry.Size(), int64(-1)) // Size can be 0 but not negative
	}
}

func TestDir_ReadDirFileInfoDirs(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	entries, err := fs.ReadDir("/testdata")
	require.NoError(t, err)
	
	// Find the subdirectory entry
	var subdirEntry os.FileInfo
	for _, entry := range entries {
		if entry.Name() == "subdir" {
			subdirEntry = entry
			break
		}
	}
	
	require.NotNil(t, subdirEntry, "subdir should be found")
	assert.True(t, subdirEntry.IsDir(), "subdir should be a directory")
	assert.Equal(t, "subdir", subdirEntry.Name())
}