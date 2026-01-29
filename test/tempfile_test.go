package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tempFS interface {
	billy.Basic
	billy.TempFile
}

func eachTempFS(t *testing.T, test func(t *testing.T, fs tempFS)) {
	t.Helper()
	for _, fs := range allFS(t.TempDir) {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			test(t, fs)
		})
	}
}

func TestTempFile(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		f, err := fs.TempFile("", "bar")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		assert.NotEqual(t, strings.Index(f.Name(), "bar"), -1)
	})
}

func TestTempFileWithPath(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		f, err := fs.TempFile("foo", "bar")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		assert.True(t, strings.HasPrefix(f.Name(), fs.Join("foo", "bar")))
	})
}

func TestTempFileFullWithPath(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		f, err := fs.TempFile("/foo", "bar")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		assert.NotEqual(t, strings.Index(f.Name(), fs.Join("foo", "bar")), -1)
	})
}

func TestRemoveTempFile(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		f, err := fs.TempFile("test-dir", "test-prefix")
		require.NoError(t, err)

		fn := f.Name()
		require.NoError(t, f.Close())
		require.NoError(t, fs.Remove(fn))
	})
}

func TestRenameTempFile(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		f, err := fs.TempFile("test-dir", "test-prefix")
		require.NoError(t, err)

		fn := f.Name()
		require.NoError(t, f.Close())
		require.NoError(t, fs.Rename(fn, "other-path"))
	})
}

func TestTempFileMany(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		for i := 0; i < 1024; i++ {
			var files []billy.File

			for j := 0; j < 100; j++ {
				f, err := fs.TempFile("test-dir", "test-prefix")
				require.NoError(t, err)
				files = append(files, f)
			}

			for _, f := range files {
				require.NoError(t, f.Close())
				require.NoError(t, fs.Remove(f.Name()))
			}
		}
	})
}

func TestTempFileManyWithUtil(t *testing.T) {
	eachTempFS(t, func(t *testing.T, fs tempFS) {
		t.Helper()
		for i := 0; i < 1024; i++ {
			var files []billy.File

			for j := 0; j < 100; j++ {
				f, err := util.TempFile(fs, "test-dir", "test-prefix")
				require.NoError(t, err)
				files = append(files, f)
			}

			for _, f := range files {
				require.NoError(t, f.Close())
				require.NoError(t, fs.Remove(f.Name()))
			}
		}
	})
}
