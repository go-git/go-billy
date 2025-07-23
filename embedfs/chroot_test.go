package embedfs

import (
	"io"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/embedfs_testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChroot_Basic(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test chroot to existing directory
	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)
	require.NotNil(t, chrootFS)

	// Test that we can access files in the chrooted filesystem
	f, err := chrootFS.Open("file1.txt")
	require.NoError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Hello from embedfs!", string(content))
}

func TestChroot_NestedDirectory(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test chroot to nested directory
	chrootFS, err := fs.Chroot("testdata/subdir")
	require.NoError(t, err)
	require.NotNil(t, chrootFS)

	// Test that we can access nested files from the chrooted root
	f, err := chrootFS.Open("nested.txt")
	require.NoError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Nested file content", string(content))
}

func TestChroot_StatInChroot(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test stat on files that exist in chrooted directory
	fi, err := chrootFS.Stat("file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "file1.txt", fi.Name())
	assert.False(t, fi.IsDir())

	// Test stat on directories that exist in chrooted directory
	fi, err = chrootFS.Stat("subdir")
	require.NoError(t, err)
	assert.Equal(t, "subdir", fi.Name())
	assert.True(t, fi.IsDir())

	// Test stat with absolute path in chrooted filesystem
	fi, err = chrootFS.Stat("/file2.txt")
	require.NoError(t, err)
	assert.Equal(t, "file2.txt", fi.Name())
	assert.False(t, fi.IsDir())
}

func TestChroot_ReadDirInChroot(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test reading directory contents from chrooted root
	entries, err := chrootFS.ReadDir("/")
	require.NoError(t, err)

	expectedFiles := []string{"empty.txt", "file1.txt", "file2.txt", "subdir"}
	assert.Len(t, entries, len(expectedFiles))

	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		foundFiles[entry.Name()] = true
	}

	for _, expected := range expectedFiles {
		assert.True(t, foundFiles[expected], "Expected file %s not found", expected)
	}

	// Test reading subdirectory from chrooted filesystem
	entries, err = chrootFS.ReadDir("subdir")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "nested.txt", entries[0].Name())
}

func TestChroot_PathNormalization(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test chroot with different path formats
	tests := []struct {
		name       string
		chrootPath string
		openPath   string
		expectFile string
	}{
		{
			name:       "absolute chroot path",
			chrootPath: "/testdata",
			openPath:   "file1.txt",
			expectFile: "file1.txt",
		},
		{
			name:       "relative chroot path",
			chrootPath: "testdata",
			openPath:   "file1.txt",
			expectFile: "file1.txt",
		},
		{
			name:       "absolute open path in chroot",
			chrootPath: "testdata",
			openPath:   "/file1.txt",
			expectFile: "file1.txt",
		},
		{
			name:       "nested chroot",
			chrootPath: "testdata/subdir",
			openPath:   "nested.txt",
			expectFile: "nested.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chrootFS, err := fs.Chroot(tt.chrootPath)
			require.NoError(t, err)

			f, err := chrootFS.Open(tt.openPath)
			require.NoError(t, err)
			defer f.Close()

			assert.Equal(t, tt.expectFile, f.Name())
		})
	}
}

func TestChroot_NonExistentPath(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test chroot to non-existent directory - billy's chroot helper allows this
	chrootFS, err := fs.Chroot("nonexistent")
	require.NoError(t, err)
	require.NotNil(t, chrootFS)

	// But accessing files within the non-existent chroot should fail
	_, err = chrootFS.Open("anyfile.txt")
	assert.Error(t, err)
}

func TestChroot_Join(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())
	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test Join operation in chrooted filesystem
	joined := chrootFS.Join("subdir", "nested.txt")
	assert.Equal(t, "subdir/nested.txt", joined)

	// Test that joined path can be used to open file
	f, err := chrootFS.Open(joined)
	require.NoError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Nested file content", string(content))
}

func TestChroot_UnsupportedOperations(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())
	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test that write operations still fail in chrooted embedfs
	_, err = chrootFS.Create("newfile.txt")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = chrootFS.Remove("file1.txt")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = chrootFS.Rename("file1.txt", "renamed.txt")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = chrootFS.MkdirAll("newdir", 0755)
	require.ErrorIs(t, err, billy.ErrReadOnly)
}

func TestChroot_NestedChroot(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())

	// Test creating nested chrootfs
	firstChroot, err := fs.Chroot("testdata")
	require.NoError(t, err)

	secondChroot, err := firstChroot.Chroot("subdir")
	require.NoError(t, err)

	// Test that nested chroot works correctly
	f, err := secondChroot.Open("nested.txt")
	require.NoError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Nested file content", string(content))

	// Test that we can't access parent directory from nested chroot
	entries, err := secondChroot.ReadDir("/")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "nested.txt", entries[0].Name())
}

func TestChroot_FileOperations(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())
	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test file operations in chrooted filesystem
	f, err := chrootFS.Open("file2.txt")
	require.NoError(t, err)
	defer f.Close()

	// Test Read
	buf := make([]byte, 10)
	n, err := f.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "Another te", string(buf[:n]))

	// Test Seek
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	// Test ReadAt
	buf2 := make([]byte, 7)
	n, err = f.ReadAt(buf2, 8)
	require.NoError(t, err)
	assert.Equal(t, "test fi", string(buf2[:n]))

	// Test that file position wasn't affected by ReadAt
	n, err = f.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "Another te", string(buf[:n]))
}

func TestChroot_Lstat(t *testing.T) {
	t.Parallel()

	fs := New(embedfs_testdata.GetTestData())
	chrootFS, err := fs.Chroot("testdata")
	require.NoError(t, err)

	// Test Lstat in chrooted filesystem (should behave same as Stat for embedfs)
	fi, err := chrootFS.Lstat("file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "file1.txt", fi.Name())
	assert.False(t, fi.IsDir())
}