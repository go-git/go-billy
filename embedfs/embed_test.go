package embedfs

import (
	"embed"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/empty.txt
var singleFile embed.FS

//go:embed testdata
var testdataDir embed.FS

var empty embed.FS

func TestOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    []byte
		wantErr bool
	}{
		{
			name: "testdata/empty.txt",
			want: []byte(""),
		},
		{
			name: "testdata/empty2.txt",
			want: []byte("test"),
		},
		{
			name:    "non-existent",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := New(&testdataDir)

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
			file:    "testdata/empty.txt",
			flag:    os.O_CREATE,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_WRONLY",
			file:    "testdata/empty.txt",
			flag:    os.O_WRONLY,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_TRUNC",
			file:    "testdata/empty.txt",
			flag:    os.O_TRUNC,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_RDWR",
			file:    "testdata/empty.txt",
			flag:    os.O_RDWR,
			wantErr: "read-only filesystem",
		},
		{
			name:    "O_EXCL",
			file:    "testdata/empty.txt",
			flag:    os.O_EXCL,
			wantErr: "read-only filesystem",
		},
		{
			name: "O_RDONLY",
			file: "testdata/empty.txt",
			flag: os.O_RDONLY,
		},
		{
			name: "no flags",
			file: "testdata/empty.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := New(&testdataDir)

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
			name: "testdata/empty.txt",
			want: "empty.txt",
		},
		{
			name: "testdata/empty2.txt",
			want: "empty2.txt",
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
			t.Parallel()
			fs := New(&testdataDir)

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
			name: "singleFile",
			path: "testdata",
			fs:   &singleFile,
			want: []string{"empty.txt"},
		},
		{
			name:    "empty",
			path:    "",
			fs:      &empty,
			want:    []string{},
			wantErr: true,
		},
		{
			name: "testdataDir w/ path",
			path: "testdata",
			fs:   &testdataDir,
			want: []string{"empty.txt", "empty2.txt"},
		},
		{
			name:    "testdataDir return no dir names",
			path:    "",
			fs:      &testdataDir,
			want:    []string{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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

	fs := New(&testdataDir)

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

	fs := New(&testdataDir)

	f, err := fs.Open("testdata/empty.txt")
	require.NoError(t, err)
	assert.NotNil(t, f)

	_, err = f.Write([]byte("foo"))
	require.ErrorIs(t, err, billy.ErrReadOnly)

	err = f.Truncate(0)
	require.ErrorIs(t, err, billy.ErrReadOnly)
}

func TestFileSeek(t *testing.T) {
	fs := New(&testdataDir)

	f, err := fs.Open("testdata/empty2.txt")
	require.NoError(t, err)
	assert.NotNil(t, f)

	tests := []struct {
		seekOff    int64
		seekWhence int
		want       string
	}{
		{seekOff: 3, seekWhence: io.SeekStart, want: ""},
		{seekOff: 3, seekWhence: io.SeekStart, want: "t"},
		{seekOff: 2, seekWhence: io.SeekStart, want: "st"},
		{seekOff: 1, seekWhence: io.SeekStart, want: "est"},
		{seekOff: 0, seekWhence: io.SeekStart, want: "test"},
		{seekOff: 0, seekWhence: io.SeekStart, want: "t"},
		{seekOff: 1, seekWhence: io.SeekCurrent, want: "s"},
		{seekOff: -2, seekWhence: io.SeekEnd, want: "st"},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			//nolint:tparallel
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
	t.Parallel()

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
			t.Parallel()
			fs := New(&empty)

			got := fs.Join(tc.path...)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCapabilities(t *testing.T) {
	fs := New(&testdataDir)
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	want := billy.ReadCapability | billy.SeekCapability
	got := billy.Capabilities(fs)
	assert.Equal(t, want, got)
}
