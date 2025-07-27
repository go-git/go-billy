package embedfs

import (
	"embed"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/embedfs/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    []byte
		wantErr bool
	}{
		{
			name: "testdata/file1.txt",
			want: []byte("Hello from embedfs!"),
		},
		{
			name: "testdata/file2.txt",
			want: []byte("Another test file"),
		},
		{
			name:    "non-existent",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(testdata.GetTestData())

			var got []byte
			f, err := fs.Open(tc.name)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, f)

				got, err = io.ReadAll(f)
				require.NoError(t, err)
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOpenFileFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		flag    int
		wantErr string
	}{
		{
			name:    "O_CREATE",
			file:    "testdata/file1.txt",
			flag:    os.O_CREATE,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_WRONLY",
			file:    "testdata/file1.txt",
			flag:    os.O_WRONLY,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_TRUNC",
			file:    "testdata/file1.txt",
			flag:    os.O_TRUNC,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_RDWR",
			file:    "testdata/file1.txt",
			flag:    os.O_RDWR,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_EXCL",
			file:    "testdata/file1.txt",
			flag:    os.O_EXCL,
			wantErr: "read-only filesystem",
		},
		{
			name: "O_RDONLY",
			file: "testdata/file1.txt",
			flag: os.O_RDONLY,
		},
		{
			name: "no flags",
			file: "testdata/file1.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(testdata.GetTestData())

			_, err := fs.OpenFile(tc.file, tc.flag, 0o700)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    string
		isDir   bool
		wantErr bool
	}{
		{
			name: "testdata/file1.txt",
			want: "file1.txt",
		},
		{
			name: "testdata/file2.txt",
			want: "file2.txt",
		},
		{
			name:    "non-existent",
			wantErr: true,
		},
		{
			name:  "testdata",
			want:  "testdata",
			isDir: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(testdata.GetTestData())

			fi, err := fs.Stat(tc.name)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, fi)

				assert.Equal(t, tc.want, fi.Name())
				assert.Equal(t, tc.isDir, fi.IsDir())
			}
		})
	}
}

func TestReadDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		fs      *embed.FS
		want    []string
		wantErr bool
	}{
		{
			name: "testdata",
			path: "testdata",
			fs:   testdata.GetTestData(),
			want: []string{"empty.txt", "file1.txt", "file2.txt", "subdir"},
		},
		{
			name:    "empty path",
			path:    "",
			fs:      testdata.GetTestData(),
			want:    []string{"testdata"},
			wantErr: false,
		},
		{
			name: "root path",
			path: "/",
			fs:   testdata.GetTestData(),
			want: []string{"testdata"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(tc.fs)

			fis, err := fs.ReadDir(tc.path)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, fis, len(tc.want))
			matched := 0

			for _, n := range fis {
				for _, w := range tc.want {
					if n.Name() == w {
						matched++
					}
				}
			}

			assert.Equal(t, len(tc.want), matched, "not all files matched")
		})
	}
}

func TestUnsupported(t *testing.T) {
	t.Parallel()

	fs := New(testdata.GetTestData())

	_, err := fs.Create("test")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = fs.Remove("test")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = fs.Rename("test", "test")
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = fs.MkdirAll("test", 0o700)
	require.ErrorIs(t, err, billy.ErrReadOnly)
}

func TestFileUnsupported(t *testing.T) {
	t.Parallel()

	fs := New(testdata.GetTestData())

	f, err := fs.Open("testdata/file1.txt")
	require.NoError(t, err)
	assert.NotNil(t, f)

	_, err = f.Write([]byte("foo"))
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = f.Truncate(0)
	require.ErrorIs(t, err, billy.ErrReadOnly)
}

func TestFileSeek(t *testing.T) {
	fs := New(testdata.GetTestData())

	f, err := fs.Open("testdata/file2.txt")
	require.NoError(t, err)
	assert.NotNil(t, f)

	tests := []struct {
		seekOff    int64
		seekWhence int
		want       string
	}{
		{seekOff: 8, seekWhence: io.SeekStart, want: "test file"},   // pos now at 17
		{seekOff: 8, seekWhence: io.SeekStart, want: "t"},          // pos now at 9  
		{seekOff: 9, seekWhence: io.SeekStart, want: "est"},        // pos now at 12
		{seekOff: 1, seekWhence: io.SeekStart, want: "nother test file"}, // pos now at 17
		{seekOff: 0, seekWhence: io.SeekStart, want: "Another test file"}, // pos now at 17
		{seekOff: 0, seekWhence: io.SeekStart, want: "A"},          // pos now at 1
		{seekOff: 0, seekWhence: io.SeekCurrent, want: "n"},        // pos now at 2
		{seekOff: -4, seekWhence: io.SeekEnd, want: "file"},        // pos now at 17
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			_, err = f.Seek(tc.seekOff, tc.seekWhence)
			require.NoError(t, err)

			data := make([]byte, len(tc.want))
			n, err := f.Read(data)
			require.NoError(t, err)
			assert.Equal(t, len(tc.want), n)
			assert.Equal(t, []byte(tc.want), data)
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want string
	}{
		{
			name: "no leading slash",
			path: []string{"data", "foo/bar"},
			want: "data/foo/bar",
		},
		{
			name: "w/ leading slash",
			path: []string{"/data", "foo/bar"},
			want: "/data/foo/bar",
		},
		{
			name: "..",
			path: []string{"/data", "../bar"},
			want: "/bar",
		},
		{
			name: ".",
			path: []string{"/data", "./bar"},
			want: "/data/bar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(testdata.GetTestData())

			got := fs.Join(tc.path...)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Additional comprehensive tests using rich test data

func TestEmbedfs_ComprehensiveOpen(t *testing.T) {
	t.Parallel()
	
	fs := New(testdata.GetTestData())
	
	// Test opening existing embedded file with content
	f, err := fs.Open("/testdata/file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "testdata/file1.txt", f.Name())
	require.NoError(t, f.Close())
}

func TestEmbedfs_ComprehensiveRead(t *testing.T) {
	t.Parallel()
	
	fs := New(testdata.GetTestData())
	
	f, err := fs.Open("/testdata/file1.txt")
	require.NoError(t, err)
	defer f.Close()
	
	// Read the actual content
	buf := make([]byte, 100)
	n, err := f.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "Hello from embedfs!", string(buf[:n]))
}

func TestEmbedfs_NestedFileOperations(t *testing.T) {
	t.Parallel()
	
	fs := New(testdata.GetTestData())
	
	// Test nested file read
	f, err := fs.Open("/testdata/subdir/nested.txt")
	require.NoError(t, err)
	defer f.Close()
	
	buf := make([]byte, 100)
	n, err := f.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "Nested file content", string(buf[:n]))
}

func TestEmbedfs_PathNormalization(t *testing.T) {
	t.Parallel()
	
	fs := New(testdata.GetTestData())
	
	// Test that our path normalization works across all methods
	tests := []struct {
		name string
		path string
	}{
		{"root", "/"},
		{"top-level", "/testdata"},
		{"nested", "/testdata/subdir"},
		{"deep file", "/testdata/subdir/nested.txt"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All these should work with our path normalization
			_, err := fs.Stat(tt.path)
			if tt.name == "deep file" {
				require.NoError(t, err, "file should exist")
			} else {
				require.NoError(t, err, "directory should exist")
			}
		})
	}
}

func TestFile_ReadAt(t *testing.T) {
	t.Parallel()

	fs := New(testdata.GetTestData())

	f, err := fs.Open("/testdata/file1.txt")
	require.NoError(t, err)
	defer f.Close()

	// Test ReadAt without affecting file position
	tests := []struct {
		name   string
		offset int64
		length int
		want   string
	}{
		{"beginning", 0, 5, "Hello"},
		{"middle", 6, 4, "from"},
		{"end", 15, 4, "dfs!"},
		{"full content", 0, 19, "Hello from embedfs!"},
		{"beyond end", 100, 10, ""},  // Should return EOF
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.length)
			n, err := f.ReadAt(buf, tt.offset)
			
			if tt.offset >= 19 {  // Beyond file size
				require.Error(t, err)
				assert.Equal(t, 0, n)
			} else {
				if tt.offset+int64(tt.length) > 19 {
					// Partial read at end of file
					require.Error(t, err) // Should be EOF
					assert.Greater(t, n, 0)
					assert.Equal(t, tt.want, string(buf[:n]))
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.length, n)
					assert.Equal(t, tt.want, string(buf[:n]))
				}
			}
		})
	}

	// Verify ReadAt doesn't change file position
	pos, err := f.Seek(0, 1) // Get current position
	require.NoError(t, err)
	assert.Equal(t, int64(0), pos, "ReadAt should not change file position")
}

func TestFile_Close(t *testing.T) {
	t.Parallel()

	fs := New(testdata.GetTestData())

	f, err := fs.Open("/testdata/file1.txt")
	require.NoError(t, err)

	// Test first close
	err = f.Close()
	require.NoError(t, err)

	// Test multiple closes (should be safe)
	err = f.Close()
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	// Note: embedfs doesn't necessarily fail operations after close
	// since embed.FS files remain readable. This tests that Close() works
	// without error, but doesn't enforce post-close failure behavior.
}

func TestFile_LockUnlock(t *testing.T) {
	t.Parallel()

	fs := New(testdata.GetTestData())

	f, err := fs.Open("/testdata/file1.txt")
	require.NoError(t, err)
	defer f.Close()

	// Lock/Unlock should be no-ops that don't error
	err = f.Lock()
	require.NoError(t, err)

	err = f.Unlock()
	require.NoError(t, err)

	// Multiple lock/unlock sequences should work
	err = f.Lock()
	require.NoError(t, err)
	err = f.Lock()
	require.NoError(t, err)
	err = f.Unlock()
	require.NoError(t, err)
	err = f.Unlock()
	require.NoError(t, err)
}
