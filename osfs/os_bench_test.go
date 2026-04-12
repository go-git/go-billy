package osfs_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/helper/iofs"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/require"
)

const fileName = "foo.bar"

type benchEnv struct {
	baseDir string
	root    *os.Root
	mem     billy.Filesystem
	chroot  billy.Filesystem
	bound   billy.Filesystem
}

func newBenchEnv(b *testing.B, withFile bool) benchEnv {
	b.Helper()
	b.StopTimer()

	baseDir := b.TempDir()
	root, err := os.OpenRoot(baseDir)
	require.NoError(b, err)

	m := memfs.New()
	if withFile {
		err = os.WriteFile(filepath.Join(baseDir, fileName), []byte("test"), 0o600)
		require.NoError(b, err)
		err = util.WriteFile(m, fileName, []byte("test"), 0o600)
		require.NoError(b, err)
	}

	b.StartTimer()
	return benchEnv{
		baseDir: baseDir,
		root:    root,
		mem:     m,
		chroot:  osfs.New(baseDir, osfs.WithChrootOS()),
		bound:   osfs.New(baseDir, osfs.WithBoundOS()),
	}
}

func BenchmarkOpen(b *testing.B) {
	e := newBenchEnv(b, true)
	b.Run("memfs", benchmarkOpen(e.mem))
	b.Run("chrootOS", benchmarkOpen(e.chroot))
	b.Run("boundOS", benchmarkOpen(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := e.root.Open(fileName)
			if err != nil {
				b.Fatal("cannot open file", "error", err)
			}
		}
	})
}

func BenchmarkCreate(b *testing.B) {
	e := newBenchEnv(b, false)
	b.Run("memfs", benchmarkCreate(e.mem))
	b.Run("chrootOS", benchmarkCreate(e.chroot))
	b.Run("boundOS", benchmarkCreate(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			f, err := e.root.Create(fileName)
			if err != nil {
				b.Fatal("cannot create file", "error", err)
			}
			f.Close()
		}
	})
}

func BenchmarkStat(b *testing.B) {
	e := newBenchEnv(b, true)
	b.Run("memfs", benchmarkStat(e.mem))
	b.Run("chrootOS", benchmarkStat(e.chroot))
	b.Run("boundOS", benchmarkStat(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := e.root.Stat(fileName)
			if err != nil {
				b.Fatal("cannot stat file", "error", err)
			}
		}
	})
}

func BenchmarkLstat(b *testing.B) {
	e := newBenchEnv(b, true)
	b.Run("memfs", benchmarkLstat(e.mem))
	b.Run("chrootOS", benchmarkLstat(e.chroot))
	b.Run("boundOS", benchmarkLstat(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := e.root.Lstat(fileName)
			if err != nil {
				b.Fatal("cannot lstat file", "error", err)
			}
		}
	})
}

func newBenchEnvMany(b *testing.B, n int) benchEnv {
	b.Helper()
	b.StopTimer()

	baseDir := b.TempDir()
	root, err := os.OpenRoot(baseDir)
	require.NoError(b, err)

	osfn := filepath.Join(baseDir, fileName)
	m := memfs.New()
	for i := range n {
		err = os.WriteFile(fmt.Sprint(osfn, i), []byte("test"), 0o600)
		require.NoError(b, err)
		err = util.WriteFile(m, fmt.Sprint(fileName, i), []byte("test"), 0o600)
		require.NoError(b, err)
	}

	b.StartTimer()
	return benchEnv{
		baseDir: baseDir,
		root:    root,
		mem:     m,
		chroot:  osfs.New(baseDir, osfs.WithChrootOS()),
		bound:   osfs.New(baseDir, osfs.WithBoundOS()),
	}
}

func BenchmarkReaddir(b *testing.B) {
	e := newBenchEnvMany(b, 1000)
	b.Run("memfs", benchmarkReaddir(e.mem, "."))
	b.Run("chrootOS", benchmarkReaddir(e.chroot, "."))
	b.Run("boundOS", benchmarkReaddir(e.bound, "."))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			_, err := os.ReadDir(e.baseDir)
			if err != nil {
				b.Fatal("cannot read dir", "error", err)
			}
		}
	})
}

func BenchmarkWalkdir(b *testing.B) {
	e := newBenchEnvMany(b, 1000)
	b.Run("memfs", benchmarkWalkdir(iofs.New(e.mem)))
	b.Run("chrootOS", benchmarkWalkdir(iofs.New(e.chroot)))
	b.Run("boundOS", benchmarkWalkdir(iofs.New(e.bound)))
	b.Run("go-lib", benchmarkWalkdir(e.root.FS()))
}

func BenchmarkRename(b *testing.B) {
	e := newBenchEnv(b, false)
	b.Run("memfs", benchmarkRename(e.mem))
	b.Run("chrootOS", benchmarkRename(e.chroot))
	b.Run("boundOS", benchmarkRename(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			f, err := e.root.Create("rename-src")
			if err != nil {
				b.Fatal(err)
			}
			f.Close()
			b.StartTimer()

			err = e.root.Rename("rename-src", "rename-dst")
			if err != nil {
				b.Fatal("cannot rename file", "error", err)
			}
		}
	})
}

func BenchmarkRemove(b *testing.B) {
	e := newBenchEnv(b, false)
	b.Run("memfs", benchmarkRemove(e.mem))
	b.Run("chrootOS", benchmarkRemove(e.chroot))
	b.Run("boundOS", benchmarkRemove(e.bound))
	b.Run("go-lib", func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			f, err := e.root.Create("remove-target")
			if err != nil {
				b.Fatal(err)
			}
			f.Close()
			b.StartTimer()

			err = e.root.Remove("remove-target")
			if err != nil {
				b.Fatal("cannot remove file", "error", err)
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

func benchmarkCreate(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			f, err := fs.Create(fileName)
			if err != nil {
				b.Fatal("cannot create file", "error", err)
			}
			f.Close()
		}
	}
}

func benchmarkStat(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			_, err := fs.Stat(fileName)
			if err != nil {
				b.Fatal("cannot stat file", "error", err)
			}
		}
	}
}

func benchmarkLstat(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			_, err := fs.Lstat(fileName)
			if err != nil {
				b.Fatal("cannot lstat file", "error", err)
			}
		}
	}
}

func benchmarkWalkdir(fsys fs.FS) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			i := 0
			err := fs.WalkDir(fsys, ".", func(_ string, _ fs.DirEntry, err error) error {
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

func benchmarkRename(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			f, err := fs.Create("rename-src")
			if err != nil {
				b.Fatal(err)
			}
			f.Close()
			b.StartTimer()

			err = fs.Rename("rename-src", "rename-dst")
			if err != nil {
				b.Fatal("cannot rename file", "error", err)
			}
		}
	}
}

func benchmarkRemove(fs billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			f, err := fs.Create("remove-target")
			if err != nil {
				b.Fatal(err)
			}
			f.Close()
			b.StartTimer()

			err = fs.Remove("remove-target")
			if err != nil {
				b.Fatal("cannot remove file", "error", err)
			}
		}
	}
}
