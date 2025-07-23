package embedfs

import (
	"testing"

	"github.com/go-git/go-billy/v6/embedfs_testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// General filesystem tests adapted from test/fs_test.go for read-only embedfs

func TestFS_ReadDir(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test basic ReadDir functionality
	entries, err := fs.ReadDir("/")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "testdata", entries[0].Name())
	assert.True(t, entries[0].IsDir())

	// Test reading a subdirectory
	entries, err = fs.ReadDir("/testdata")
	require.NoError(t, err)
	assert.Equal(t, 4, len(entries), "testdata should contain 4 entries")
	
	// Verify we can find expected files
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	assert.Contains(t, names, "empty.txt")
	assert.Contains(t, names, "file1.txt")
	assert.Contains(t, names, "file2.txt")
	assert.Contains(t, names, "subdir")
}

func TestFS_ReadDirNonExistent(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test reading non-existent directory
	_, err := fs.ReadDir("/non-existent")
	require.Error(t, err)
}

func TestFS_StatExisting(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test stating existing file
	fi, err := fs.Stat("/testdata/file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "file1.txt", fi.Name())
	assert.False(t, fi.IsDir())
	assert.Greater(t, fi.Size(), int64(0))

	// Test stating existing directory
	fi, err = fs.Stat("/testdata")
	require.NoError(t, err)
	assert.Equal(t, "testdata", fi.Name())
	assert.True(t, fi.IsDir())
}

func TestFS_StatNonExistent(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test stating non-existent file
	_, err := fs.Stat("/non-existent")
	require.Error(t, err)
}


func TestFS_EmptyFileHandling(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test empty file stat
	fi, err := fs.Stat("/testdata/empty.txt")
	require.NoError(t, err)
	assert.Equal(t, "empty.txt", fi.Name())
	assert.False(t, fi.IsDir())
	assert.Equal(t, int64(0), fi.Size())

	// Test opening empty file
	f, err := fs.Open("/testdata/empty.txt")
	require.NoError(t, err)
	defer f.Close()

	// Test reading from empty file
	buf := make([]byte, 10)
	n, err := f.Read(buf)
	require.Error(t, err) // Should be EOF
	assert.Equal(t, 0, n)

	// Test ReadAt on empty file
	n, err = f.ReadAt(buf, 0)
	require.Error(t, err) // Should be EOF
	assert.Equal(t, 0, n)
}

