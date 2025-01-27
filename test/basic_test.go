package test

import (
	"bytes"
	"fmt"
	"io"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func eachBasicFS(t *testing.T, test func(t *testing.T, fs Basic)) {
	t.Helper()

	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestCreate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Create("foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", f.Name())
		require.NoError(t, f.Close())
	})
}

func TestCreateDepth(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Create("bar/foo")
		require.NoError(t, err)
		assert.Equal(t, fs.Join("bar", "foo"), f.Name())
		require.NoError(t, f.Close())
	})
}

func TestCreateDepthAbsolute(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Create("/bar/foo")
		require.NoError(t, err)
		assert.Equal(t, fs.Join("bar", "foo"), f.Name())
		require.NoError(t, f.Close())
	})
}

func TestCreateOverwrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		for i := 0; i < 3; i++ {
			f, err := fs.Create("foo")
			require.NoError(t, err)

			l, err := f.Write([]byte(fmt.Sprintf("foo%d", i)))
			require.NoError(t, err)
			assert.Equal(t, 4, l)

			err = f.Close()
			require.NoError(t, err)
		}

		f, err := fs.Open("foo")
		require.NoError(t, err)

		wrote, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "foo2", string(wrote))
		require.NoError(t, f.Close())
	})
}

func TestCreateAndClose(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Create("foo")
		require.NoError(t, err)

		_, err = f.Write([]byte("foo"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = fs.Open(f.Name())
		require.NoError(t, err)

		wrote, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "foo", string(wrote))
		require.NoError(t, f.Close())
	})
}

func TestOpen(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Create("foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", f.Name())
		require.NoError(t, f.Close())

		f, err = fs.Open("foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", f.Name())
		require.NoError(t, f.Close())
	})
}

func TestOpenNotExists(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.Open("not-exists")
		assert.NotNil(t, err)
		assert.Nil(t, f)
	})
}

func TestOpenFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		require.NoError(t, err)
		testWriteClose(t, f, "foo1")

		// Truncate if it exists
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "foo1overwritten")

		// Read-only if it exists
		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testReadClose(t, f, "foo1overwritten")

		// Create when it does exist
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "bar")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		require.NoError(t, err)
		testReadClose(t, f, "bar")
	})
}

func TestOpenFileNoTruncate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		defaultMode := os.FileMode(0666)

		// Create when it does not exist
		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "foo1")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		require.NoError(t, err)
		testReadClose(t, f, "foo1")

		// Create when it does exist
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "bar")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		require.NoError(t, err)
		testReadClose(t, f, "bar1")
	})
}

func TestOpenFileAppend(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "foo1")

		f, err = fs.OpenFile("foo1", os.O_WRONLY|os.O_APPEND, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())
		testWriteClose(t, f, "bar1")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		require.NoError(t, err)
		testReadClose(t, f, "foo1bar1")
	})
}

func TestOpenFileReadWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())

		written, err := f.Write([]byte("foobar"))
		assert.Equal(t, 6, written)
		require.NoError(t, err)

		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)

		written, err = f.Write([]byte("qux"))
		assert.Equal(t, 3, written)
		require.NoError(t, err)

		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)

		testReadClose(t, f, "quxbar")
	})
}

func TestOpenFileWithModes(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()

		f, err := fs.OpenFile("foo", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, customMode)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		fi, err := fs.Stat("foo")
		require.NoError(t, err)
		assert.Equal(t, customMode, fi.Mode())
	})
}

func testWriteClose(t *testing.T, f File, content string) {
	t.Helper()

	written, err := f.Write([]byte(content))
	assert.Equal(t, len(content), written)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func testReadClose(t *testing.T, f File, content string) {
	t.Helper()
	read, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, string(read), content)
	require.NoError(t, f.Close())
}

func TestFileWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)

		n, err := f.Write([]byte("foo"))
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)

		all, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "foo", string(all))
		require.NoError(t, f.Close())
	})
}

func TestFileWriteClose(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)

		require.NoError(t, f.Close())

		_, err = f.Write([]byte("foo"))
		assert.NotNil(t, err)
	})
}

func TestFileRead(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)

		all, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "foo", string(all))
		require.NoError(t, f.Close())
	})
}

func TestFileClosed(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = io.ReadAll(f)
		assert.NotNil(t, err)
	})
}

func TestFileNonRead(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		require.NoError(t, err)

		f, err := fs.OpenFile("foo", os.O_WRONLY, 0)
		require.NoError(t, err)

		_, err = io.ReadAll(f)
		assert.NotNil(t, err)

		require.NoError(t, f.Close())
	})
}

func TestFileSeekstart(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		testFileSeek(t, fs, 10, io.SeekStart)
	})
}

func TestFileSeekCurrent(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		testFileSeek(t, fs, 5, io.SeekCurrent)
	})
}

func TestFileSeekEnd(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		testFileSeek(t, fs, -26, io.SeekEnd)
	})
}

func testFileSeek(t *testing.T, fs Basic, offset int64, whence int) {
	t.Helper()
	err := util.WriteFile(fs, "foo", []byte("0123456789abcdefghijklmnopqrstuvwxyz"), 0644)
	require.NoError(t, err)

	f, err := fs.Open("foo")
	require.NoError(t, err)

	some := make([]byte, 5)
	_, err = f.Read(some)
	require.NoError(t, err)
	assert.Equal(t, "01234", string(some))

	p, err := f.Seek(offset, whence)
	require.NoError(t, err)
	assert.Equal(t, 10, int(p))

	all, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Len(t, all, 26)
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz", string(all))
	require.NoError(t, f.Close())
}

func TestSeekToEndAndWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
		require.NoError(t, err)
		assert.Equal(t, "foo1", f.Name())

		_, err = f.Seek(10, io.SeekEnd)
		require.NoError(t, err)

		n, err := f.Write([]byte(`TEST`))
		require.NoError(t, err)
		assert.Equal(t, 4, n)

		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)

		testReadClose(t, f, "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00TEST")
	})
}

func TestFileSeekClosed(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = f.Seek(0, 0)
		assert.NotNil(t, err)
	})
}

func TestFileCloseTwice(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)

		require.NoError(t, f.Close())
		assert.Error(t, f.Close())
	})
}

func TestStat(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)
		require.NoError(t, err)

		fi, err := fs.Stat("foo/bar")
		require.NoError(t, err)
		assert.Equal(t, "bar", fi.Name())
		assert.Equal(t, int64(3), fi.Size())
		assert.Equal(t, customMode, fi.Mode())
		assert.False(t, fi.ModTime().IsZero())
		assert.False(t, fi.IsDir())
	})
}

func TestStatNonExistent(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		fi, err := fs.Stat("non-existent")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, fi)
	})
}

func TestRename(t *testing.T) {
	tests := []struct {
		name      string
		before    func(*testing.T, billy.Filesystem)
		from      string
		to        string
		wantErr   *error
		wantFiles []string
	}{
		{
			name:    "from not found",
			from:    "foo",
			to:      "bar",
			wantErr: &os.ErrNotExist,
		},
		{
			name: "file rename",
			before: func(t *testing.T, fs billy.Filesystem) {
				root := fsRoot(fs)
				f, err := fs.Create(fs.Join(root, "foo"))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			},
			from:      "foo",
			to:        "bar",
			wantFiles: []string{filepath.FromSlash("/bar")},
		},
		{
			name: "dir rename",
			before: func(t *testing.T, fs billy.Filesystem) {
				root := fsRoot(fs)
				f, err := fs.Create(fs.Join(root, "foo", "bar1"))
				require.NoError(t, err)
				require.NoError(t, f.Close())
				f, err = fs.Create(fs.Join(root, "foo", "bar2"))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			},
			from: "foo",
			to:   "bar",
			wantFiles: []string{
				filepath.FromSlash("/bar/bar1"),
				filepath.FromSlash("/bar/bar2")},
		},
	}

	eachFS(t, func(t *testing.T, fs Filesystem) {
		t.Helper()

		root := fsRoot(fs)
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if tc.before != nil {
					tc.before(t, fs)
				}

				err := fs.Rename(fs.Join(root, tc.from), fs.Join(root, tc.to))
				if tc.wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, *tc.wantErr)
				}

				err = util.Walk(fs, root, func(path string, fi stdfs.FileInfo, err error) error {
					if err != nil {
						return err
					}

					if fi.IsDir() {
						return nil
					}

					if filepath.Dir(root) == "" {
						path = strings.TrimPrefix(path, root)
					}
					if !slices.Contains(tc.wantFiles, path) {
						assert.Fail(t, "file not found", "name", path)
					}

					return nil
				})
				require.NoError(t, err)

				fis, _ := fs.ReadDir(root)
				for _, fi := range fis {
					cpath := fs.Join(root, fi.Name())
					err := util.RemoveAll(fs, cpath)
					require.NoError(t, err)
				}
			})
		}
	})
}

func fsRoot(fs billy.Filesystem) string {
	if reflect.TypeOf(fs) == reflect.TypeOf(&osfs.BoundOS{}) {
		return fs.Root()
	}
	return string(filepath.Separator)
}

func TestOpenAndWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", nil, 0644)
		require.NoError(t, err)

		foo, err := fs.Open("foo")
		assert.NotNil(t, foo)
		require.NoError(t, err)

		n, err := foo.Write([]byte("foo"))
		assert.NotNil(t, err)
		assert.Equal(t, 0, n)

		require.NoError(t, foo.Close())
	})
}

func TestOpenAndStat(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		require.NoError(t, err)

		foo, err := fs.Open("foo")
		assert.NotNil(t, foo)
		assert.Equal(t, "foo", foo.Name())
		require.NoError(t, err)
		require.NoError(t, foo.Close())

		stat, err := fs.Stat("foo")
		assert.NotNil(t, stat)
		assert.NoError(t, err)
		assert.Equal(t, "foo", stat.Name())
		assert.Equal(t, int64(3), stat.Size())
	})
}

func TestRemove(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		err = fs.Remove("foo")
		require.NoError(t, err)
	})
}

func TestRemoveNonExisting(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := fs.Remove("NON-EXISTING")
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestRemoveNotEmptyDir(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", nil, 0644)
		require.NoError(t, err)

		err = fs.Remove("no-exists")
		assert.NotNil(t, err)
	})
}

func TestJoin(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		assert.Equal(t, fs.Join("foo", "bar"), fmt.Sprintf("foo%cbar", filepath.Separator))
	})
}

func TestReadAtOnReadWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)
		_, err = f.Write([]byte("abcdefg"))
		require.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		b := make([]byte, 3)
		n, err := rf.ReadAt(b, 2)
		require.NoError(t, err)
		assert.Equal(t, n, 3)
		assert.Equal(t, string(b), "cde")
		require.NoError(t, f.Close())
	})
}

func TestReadAtOnReadOnly(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("abcdefg"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		b := make([]byte, 3)
		n, err := rf.ReadAt(b, 2)
		require.NoError(t, err)
		assert.Equal(t, n, 3)
		assert.Equal(t, string(b), "cde")
		require.NoError(t, f.Close())
	})
}

func TestReadAtEOF(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("TEST"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)

		b := make([]byte, 5)
		n, err := f.ReadAt(b, 0)
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(b), "TEST\x00")

		err = f.Close()
		require.NoError(t, err)
	})
}

func TestReadAtOffset(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("TEST"), 0644)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		o, err := f.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		assert.Equal(t, o, int64(0))

		b := make([]byte, 4)
		n, err := rf.ReadAt(b, 0)
		require.NoError(t, err)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(b), "TEST")

		o, err = f.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		assert.Equal(t, o, int64(0))

		err = f.Close()
		require.NoError(t, err)
	})
}

func TestReadWriteLargeFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)

		size := 1 << 20

		n, err := f.Write(bytes.Repeat([]byte("F"), size))
		require.NoError(t, err)
		assert.Equal(t, n, size)

		require.NoError(t, f.Close())

		f, err = fs.Open("foo")
		require.NoError(t, err)
		b, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, len(b), size)
		require.NoError(t, f.Close())
	})
}

func TestWriteFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		err := util.WriteFile(fs, "foo", []byte("bar"), 0777)
		require.NoError(t, err)

		f, err := fs.Open("foo")
		require.NoError(t, err)

		wrote, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, string(wrote), "bar")

		require.NoError(t, f.Close())
	})
}

func TestTruncate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		t.Helper()
		f, err := fs.Create("foo")
		require.NoError(t, err)

		for _, sz := range []int64{4, 7, 2, 30, 0, 1} {
			err = f.Truncate(sz)
			require.NoError(t, err)

			bs, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, len(bs), int(sz))

			_, err = f.Seek(0, io.SeekStart)
			require.NoError(t, err)
		}

		require.NoError(t, f.Close())
	})
}
