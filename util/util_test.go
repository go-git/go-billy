package util_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempFile(t *testing.T) {
	fs := memfs.New()

	dir, err := util.TempDir(fs, "", "util_test")
	if err != nil {
		t.Fatal(err)
	}
	defer util.RemoveAll(fs, dir) //nolint

	f, err := util.TempFile(fs, dir, "foo")
	if f == nil || err != nil {
		t.Errorf("TempFile(%q, `foo`) = %v, %v", dir, f, err)
	}
}

func TestTempDir_WithDir(t *testing.T) {
	fs := memfs.New()

	dir := os.TempDir()
	name, err := util.TempDir(fs, dir, "util_test")
	if name == "" || err != nil {
		t.Errorf("TempDir(dir, `util_test`) = %v, %v", name, err)
	}
	if name != "" {
		err = util.RemoveAll(fs, name)
		require.NoError(t, err)
		re := regexp.MustCompile("^" + regexp.QuoteMeta(filepath.Join(dir, "util_test")) + "[0-9]+$")
		if !re.MatchString(name) {
			t.Errorf("TempDir(`"+dir+"`, `util_test`) created bad name %s", name)
		}
	}
}

func TestReadFile(t *testing.T) {
	fs := memfs.New()
	f, err := util.TempFile(fs, "", "")
	require.NoError(t, err)

	_, err = f.Write([]byte("foo"))
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	data, err := util.ReadFile(fs, f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "foo" || err != nil {
		t.Errorf("ReadFile(%q, %q) = %v, %v", fs, f.Name(), data, err)
	}
}

func TestReadFileCases(t *testing.T) {
	t.Parallel()

	memFile := func(name string, content []byte) func(t *testing.T) (billy.Basic, string) {
		return func(t *testing.T) (billy.Basic, string) {
			t.Helper()
			fs := memfs.New()
			require.NoError(t, util.WriteFile(fs, name, content, 0o644))
			return fs, name
		}
	}

	type readFileCase struct {
		name    string
		setup   func(t *testing.T) (billy.Basic, string)
		want    []byte
		wantErr error
	}

	tests := []readFileCase{
		{name: "empty", setup: memFile("empty", nil), want: []byte{}},
		{name: "binary", setup: memFile("bin", []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff, 0x00}), want: []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff, 0x00}},
		{
			name: "missing",
			setup: func(t *testing.T) (billy.Basic, string) {
				t.Helper()
				return memfs.New(), "missing"
			},
			wantErr: os.ErrNotExist,
		},
		{
			name: "symlink",
			setup: func(t *testing.T) (billy.Basic, string) {
				t.Helper()
				fs := memfs.New()
				require.NoError(t, util.WriteFile(fs, "target", []byte("hello"), 0o644))
				require.NoError(t, fs.Symlink("target", "link"))
				return fs, "link"
			},
			want: []byte("hello"),
		},
		{
			name: "os fs",
			setup: func(t *testing.T) (billy.Basic, string) {
				t.Helper()
				dir := t.TempDir()
				content := bytes.Repeat([]byte("xyz"), 1024)
				require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), content, 0o644))
				return osfs.New(dir), "file"
			},
			want: bytes.Repeat([]byte("xyz"), 1024),
		},
	}

	for _, size := range []int{1, 511, 512, 513, 1024, 4096, 1 << 16} {
		content := bytes.Repeat([]byte{'a'}, size)
		tests = append(tests, readFileCase{
			name:  fmt.Sprintf("%d bytes", size),
			setup: memFile("f", content),
			want:  content,
		})
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs, name := tt.setup(t)
			data, err := util.ReadFile(fs, name)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, data)
		})
	}
}

func TestTempDir(t *testing.T) {
	fs := memfs.New()
	f, err := util.TempDir(fs, "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = filepath.Rel(os.TempDir(), f)
	if err != nil {
		t.Errorf(`TempDir(fs, "", "") = %s, should be relative to os.TempDir if root filesystem`, f)
	}
}

func TestTempDir_WithNonRoot(t *testing.T) {
	fs := memfs.New()
	fs, _ = fs.Chroot("foo")
	f, err := util.TempDir(fs, "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = filepath.Rel(os.TempDir(), f)
	if err == nil {
		t.Errorf(`TempDir(fs, "", "") = %s, should not be relative to os.TempDir on not root filesystem`, f)
	}
}

func TestWriteFile_Sync(t *testing.T) {
	fs := &test.BasicMock{}
	filename := "TestWriteFile.txt"
	data := []byte("hello world")
	err := util.WriteFile(fs, filename, data, 0o644)
	require.NoError(t, err)

	assert.Len(t, fs.CallLogger.Calls, 1)
	assert.Equal(t, "Sync TestWriteFile.txt", fs.CallLogger.Calls[0])
}

func TestRemoveAllWithScopedFilesystems(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (billy.Basic, string, func(t *testing.T))
		wantErr error
	}{
		{
			name: "bound os sibling file",
			setup: func(t *testing.T) (billy.Basic, string, func(t *testing.T)) {
				t.Helper()

				tmp := t.TempDir()
				base := filepath.Join(tmp, "base")
				require.NoError(t, os.MkdirAll(base, 0o755))

				sibling := filepath.Join(tmp, "sibling")
				require.NoError(t, os.WriteFile(sibling, []byte("keep"), 0o644))

				return osfs.New(base), "../sibling", func(t *testing.T) {
					t.Helper()
					data, err := os.ReadFile(sibling)
					require.NoError(t, err)
					assert.Equal(t, []byte("keep"), data)
				}
			},
		},
		{
			name: "nested memory parent file",
			setup: func(t *testing.T) (billy.Basic, string, func(t *testing.T)) {
				t.Helper()

				root := memfs.New()
				require.NoError(t, util.WriteFile(root, "/parent-file", []byte("keep"), 0o644))

				sub, err := root.Chroot("/sub")
				require.NoError(t, err)

				return sub, "../parent-file", func(t *testing.T) {
					t.Helper()
					data, err := util.ReadFile(root, "/parent-file")
					require.NoError(t, err)
					assert.Equal(t, []byte("keep"), data)
				}
			},
			wantErr: billy.ErrCrossedBoundary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, path, verify := tt.setup(t)
			err := util.RemoveAll(fs, path)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			verify(t)
		})
	}
}
