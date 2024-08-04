package test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"
)

func eachBasicFS(t *testing.T, test func(t *testing.T, fs Basic)) {
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestCreate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)
		assert.Equal(t, "foo", f.Name())
		assert.NoError(t, f.Close())
	})
}

func TestCreateDepth(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("bar/foo")
		assert.NoError(t, err)
		assert.Equal(t, fs.Join("bar", "foo"), f.Name())
		assert.NoError(t, f.Close())
	})
}

func TestCreateDepthAbsolute(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("/bar/foo")
		assert.NoError(t, err)
		assert.Equal(t, fs.Join("bar", "foo"), f.Name())
		assert.NoError(t, f.Close())
	})
}

func TestCreateOverwrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		for i := 0; i < 3; i++ {
			f, err := fs.Create("foo")
			assert.NoError(t, err)

			l, err := f.Write([]byte(fmt.Sprintf("foo%d", i)))
			assert.NoError(t, err)
			assert.Equal(t, l, 4)

			err = f.Close()
			assert.NoError(t, err)
		}

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		wrote, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(wrote), "foo2")
		assert.NoError(t, f.Close())
	})
}

func TestCreateAndClose(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		_, err = f.Write([]byte("foo"))
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		f, err = fs.Open(f.Name())
		assert.NoError(t, err)

		wrote, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(wrote), "foo")
		assert.NoError(t, f.Close())
	})
}

func TestOpen(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo")
		assert.NoError(t, f.Close())

		f, err = fs.Open("foo")
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo")
		assert.NoError(t, f.Close())
	})
}

func TestOpenNotExists(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Open("not-exists")
		assert.NotNil(t, err)
		assert.Nil(t, f)
	})
}

func TestOpenFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		assert.NoError(t, err)
		testWriteClose(t, f, "foo1")

		// Truncate if it exists
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "foo1overwritten")

		// Read-only if it exists
		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testReadClose(t, f, "foo1overwritten")

		// Create when it does exist
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "bar")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		assert.NoError(t, err)
		testReadClose(t, f, "bar")
	})
}

func TestOpenFileNoTruncate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		defaultMode := os.FileMode(0666)

		// Create when it does not exist
		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "foo1")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		assert.NoError(t, err)
		testReadClose(t, f, "foo1")

		// Create when it does exist
		f, err = fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "bar")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		assert.NoError(t, err)
		testReadClose(t, f, "bar1")
	})
}

func TestOpenFileAppend(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "foo1")

		f, err = fs.OpenFile("foo1", os.O_WRONLY|os.O_APPEND, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")
		testWriteClose(t, f, "bar1")

		f, err = fs.OpenFile("foo1", os.O_RDONLY, defaultMode)
		assert.NoError(t, err)
		testReadClose(t, f, "foo1bar1")
	})
}

func TestOpenFileReadWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")

		written, err := f.Write([]byte("foobar"))
		assert.Equal(t, written, 6)
		assert.NoError(t, err)

		_, err = f.Seek(0, io.SeekStart)
		assert.NoError(t, err)

		written, err = f.Write([]byte("qux"))
		assert.Equal(t, written, 3)
		assert.NoError(t, err)

		_, err = f.Seek(0, io.SeekStart)
		assert.NoError(t, err)

		testReadClose(t, f, "quxbar")
	})
}

func TestOpenFileWithModes(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.OpenFile("foo", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, customMode)
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		fi, err := fs.Stat("foo")
		assert.NoError(t, err)
		assert.Equal(t, fi.Mode(), os.FileMode(customMode))
	})
}

func testWriteClose(t *testing.T, f File, content string) {
	written, err := f.Write([]byte(content))
	assert.Equal(t, written, len(content))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
}

func testReadClose(t *testing.T, f File, content string) {
	read, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, string(read), content)
	assert.NoError(t, f.Close())
}

func TestFileWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		n, err := f.Write([]byte("foo"))
		assert.NoError(t, err)
		assert.Equal(t, n, 3)

		f.Seek(0, io.SeekStart)
		all, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(all), "foo")
		assert.NoError(t, f.Close())
	})
}

func TestFileWriteClose(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		assert.NoError(t, f.Close())

		_, err = f.Write([]byte("foo"))
		assert.NotNil(t, err)
	})
}

func TestFileRead(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		all, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(all), "foo")
		assert.NoError(t, f.Close())
	})
}

func TestFileClosed(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		_, err = io.ReadAll(f)
		assert.NotNil(t, err)
	})
}

func TestFileNonRead(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		assert.NoError(t, err)

		f, err := fs.OpenFile("foo", os.O_WRONLY, 0)
		assert.NoError(t, err)

		_, err = io.ReadAll(f)
		assert.NotNil(t, err)

		assert.NoError(t, f.Close())
	})
}

func TestFileSeekstart(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		testFileSeek(t, fs, 10, io.SeekStart)
	})
}

func TestFileSeekCurrent(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		testFileSeek(t, fs, 5, io.SeekCurrent)
	})
}

func TestFileSeekEnd(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		testFileSeek(t, fs, -26, io.SeekEnd)
	})
}

func testFileSeek(t *testing.T, fs Basic, offset int64, whence int) {
	err := util.WriteFile(fs, "foo", []byte("0123456789abcdefghijklmnopqrstuvwxyz"), 0644)
	assert.NoError(t, err)

	f, err := fs.Open("foo")
	assert.NoError(t, err)

	some := make([]byte, 5)
	_, err = f.Read(some)
	assert.NoError(t, err)
	assert.Equal(t, string(some), "01234")

	p, err := f.Seek(offset, whence)
	assert.NoError(t, err)
	assert.Equal(t, int(p), 10)

	all, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Len(t, all, 26)
	assert.Equal(t, string(all), "abcdefghijklmnopqrstuvwxyz")
	assert.NoError(t, f.Close())
}

func TestSeekToEndAndWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		defaultMode := os.FileMode(0666)

		f, err := fs.OpenFile("foo1", os.O_CREATE|os.O_TRUNC|os.O_RDWR, defaultMode)
		assert.NoError(t, err)
		assert.Equal(t, f.Name(), "foo1")

		_, err = f.Seek(10, io.SeekEnd)
		assert.NoError(t, err)

		n, err := f.Write([]byte(`TEST`))
		assert.NoError(t, err)
		assert.Equal(t, n, 4)

		_, err = f.Seek(0, io.SeekStart)
		assert.NoError(t, err)

		testReadClose(t, f, "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00TEST")
	})
}

func TestFileSeekClosed(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		_, err = f.Seek(0, 0)
		assert.NotNil(t, err)
	})
}

func TestFileCloseTwice(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		assert.NoError(t, f.Close())
		assert.Error(t, f.Close())
	})
}

func TestStat(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		util.WriteFile(fs, "foo/bar", []byte("foo"), customMode)

		fi, err := fs.Stat("foo/bar")
		assert.NoError(t, err)
		assert.Equal(t, fi.Name(), "bar")
		assert.Equal(t, fi.Size(), int64(3))
		assert.Equal(t, fi.Mode(), customMode)
		assert.Equal(t, fi.ModTime().IsZero(), false)
		assert.Equal(t, fi.IsDir(), false)
	})
}

func TestStatNonExistent(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		fi, err := fs.Stat("non-existent")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, fi)
	})
}

func TestRename(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", nil, 0644)
		assert.NoError(t, err)

		err = fs.Rename("foo", "bar")
		assert.NoError(t, err)

		foo, err := fs.Stat("foo")
		assert.Nil(t, foo)
		assert.ErrorIs(t, err, os.ErrNotExist)

		bar, err := fs.Stat("bar")
		assert.NoError(t, err)
		assert.NotNil(t, bar)
	})
}

func TestOpenAndWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", nil, 0644)
		assert.NoError(t, err)

		foo, err := fs.Open("foo")
		assert.NotNil(t, foo)
		assert.NoError(t, err)

		n, err := foo.Write([]byte("foo"))
		assert.NotNil(t, err)
		assert.Equal(t, n, 0)

		assert.NoError(t, foo.Close())
	})
}

func TestOpenAndStat(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("foo"), 0644)
		assert.NoError(t, err)

		foo, err := fs.Open("foo")
		assert.NotNil(t, foo)
		assert.Equal(t, "foo", foo.Name())
		assert.NoError(t, err)
		assert.NoError(t, foo.Close())

		stat, err := fs.Stat("foo")
		assert.NotNil(t, stat)
		assert.NoError(t, err)
		assert.Equal(t, stat.Name(), "foo")
		assert.Equal(t, stat.Size(), int64(3))
	})
}

func TestRemove(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)
		assert.NoError(t, f.Close())

		err = fs.Remove("foo")
		assert.NoError(t, err)
	})
}

func TestRemoveNonExisting(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := fs.Remove("NON-EXISTING")
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestRemoveNotEmptyDir(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", nil, 0644)
		assert.NoError(t, err)

		err = fs.Remove("no-exists")
		assert.NotNil(t, err)
	})
}

func TestJoin(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		assert.Equal(t, fs.Join("foo", "bar"), fmt.Sprintf("foo%cbar", filepath.Separator))
	})
}

func TestReadAtOnReadWrite(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)
		_, err = f.Write([]byte("abcdefg"))
		assert.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		b := make([]byte, 3)
		n, err := rf.ReadAt(b, 2)
		assert.NoError(t, err)
		assert.Equal(t, n, 3)
		assert.Equal(t, string(b), "cde")
		assert.NoError(t, f.Close())
	})
}

func TestReadAtOnReadOnly(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("abcdefg"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		b := make([]byte, 3)
		n, err := rf.ReadAt(b, 2)
		assert.NoError(t, err)
		assert.Equal(t, n, 3)
		assert.Equal(t, string(b), "cde")
		assert.NoError(t, f.Close())
	})
}

func TestReadAtEOF(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("TEST"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		b := make([]byte, 5)
		n, err := f.ReadAt(b, 0)
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(b), "TEST\x00")

		err = f.Close()
		assert.NoError(t, err)
	})
}

func TestReadAtOffset(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("TEST"), 0644)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		rf, ok := f.(io.ReaderAt)
		assert.True(t, ok)

		o, err := f.Seek(0, io.SeekCurrent)
		assert.NoError(t, err)
		assert.Equal(t, o, int64(0))

		b := make([]byte, 4)
		n, err := rf.ReadAt(b, 0)
		assert.NoError(t, err)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(b), "TEST")

		o, err = f.Seek(0, io.SeekCurrent)
		assert.NoError(t, err)
		assert.Equal(t, o, int64(0))

		err = f.Close()
		assert.NoError(t, err)
	})
}

func TestReadWriteLargeFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		size := 1 << 20

		n, err := f.Write(bytes.Repeat([]byte("F"), size))
		assert.NoError(t, err)
		assert.Equal(t, n, size)

		assert.NoError(t, f.Close())

		f, err = fs.Open("foo")
		assert.NoError(t, err)
		b, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, len(b), size)
		assert.NoError(t, f.Close())
	})
}

func TestWriteFile(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		err := util.WriteFile(fs, "foo", []byte("bar"), 0777)
		assert.NoError(t, err)

		f, err := fs.Open("foo")
		assert.NoError(t, err)

		wrote, err := io.ReadAll(f)
		assert.NoError(t, err)
		assert.Equal(t, string(wrote), "bar")

		assert.NoError(t, f.Close())
	})
}

func TestTruncate(t *testing.T) {
	eachBasicFS(t, func(t *testing.T, fs Basic) {
		f, err := fs.Create("foo")
		assert.NoError(t, err)

		for _, sz := range []int64{4, 7, 2, 30, 0, 1} {
			err = f.Truncate(sz)
			assert.NoError(t, err)

			bs, err := io.ReadAll(f)
			assert.NoError(t, err)
			assert.Equal(t, len(bs), int(sz))

			_, err = f.Seek(0, io.SeekStart)
			assert.NoError(t, err)
		}

		assert.NoError(t, f.Close())
	})
}
