package osfs_test

import (
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/require"
)

const fileName = "foo.bar"

func BenchmarkOpen(b *testing.B) {
	b.StopTimer()
	baseDir := b.TempDir()
	root, err := os.OpenRoot(baseDir)
	require.NoError(b, err)

	err = root.WriteFile(fileName, []byte("test"), 0o600)
	require.NoError(b, err)

	m := memfs.New()
	err = util.WriteFile(m, fileName, []byte("test"), 0o600)
	require.NoError(b, err)
	b.StartTimer()

	b.Run("memfs", benchmarkOpen(m))
	b.Run("chrootOS", benchmarkOpen(osfs.New(baseDir, osfs.WithChrootOS())))
	b.Run("boundOS", benchmarkOpen(osfs.New(baseDir, osfs.WithBoundOS())))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := root.Open(fileName)
			if err != nil {
				b.Fatal("cannot open file", "error", err)
			}
		}
	})
}

func BenchmarkReaddir(b *testing.B) {
	b.StopTimer()
	baseDir := b.TempDir()
	root, err := os.OpenRoot(baseDir)
	require.NoError(b, err)

	m := memfs.New()
	for i := 0; i < 1000; i++ {
		err = root.WriteFile(fmt.Sprint(fileName, i), []byte("test"), 0o600)
		require.NoError(b, err)

		err = util.WriteFile(m, fmt.Sprint(fileName, i), []byte("test"), 0o600)
		require.NoError(b, err)
	}
	b.StartTimer()

	b.Run("memfs", benchmarkReaddir(m, "."))
	b.Run("chrootOS", benchmarkReaddir(osfs.New(baseDir, osfs.WithChrootOS()), "."))
	b.Run("boundOS", benchmarkReaddir(osfs.New(baseDir, osfs.WithBoundOS()), "."))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := os.ReadDir(baseDir)
			if err != nil {
				b.Fatal("cannot read dir", "error", err)
			}
		}
	})
}

func BenchmarkWalkdir(b *testing.B) {
	b.StopTimer()
	baseDir := b.TempDir()
	root, err := os.OpenRoot(baseDir)
	require.NoError(b, err)

	m := memfs.New()
	for i := 0; i < 1000; i++ {
		err = root.WriteFile(fmt.Sprint(fileName, i), []byte("test"), 0o600)
		require.NoError(b, err)

		err = util.WriteFile(m, fmt.Sprint(fileName, i), []byte("test"), 0o600)
		require.NoError(b, err)
	}
	b.StartTimer()

	b.Run("memfs", benchmarkReaddir(m, "."))
	b.Run("chrootOS", benchmarkReaddir(osfs.New(baseDir, osfs.WithChrootOS()), "."))
	b.Run("boundOS", benchmarkReaddir(osfs.New(baseDir, osfs.WithBoundOS()), "."))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			i := 0
			err := fs.WalkDir(root.FS(), ".", func(_ string, _ fs.DirEntry, err error) error {
				i++
				return err
			})
			if err != nil {
				b.Fatal("cannot walk dir", "error", err)
			}

			if i != 1001 { // 1000 files + dir entry
				b.Fatal("wrong walk number", "i", i)
			}
		}
	})
}

func benchmarkOpen(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			_, err := fs.Open(fileName)
			if err != nil {
				b.Fatal("cannot open file", "error", err)
			}
		}
	}
}

func benchmarkReaddir(fs billy.Filesystem, path string) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			fi, err := fs.ReadDir(path)
			if err != nil {
				b.Fatal("cannot read dir", "error", err)
			}
			if len(fi) != 1000 {
				b.Fatal("missing files", "len", len(fi))
			}
		}
	}
}
