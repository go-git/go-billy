//go:build !wasm

package osfs

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/require"
)

func writePattern(t *testing.T, path string, size int) []byte {
	t.Helper()
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte(i % 251)
	}
	require.NoError(t, os.WriteFile(path, buf, 0o600))
	return buf
}

func runConcurrentReadAt(t *testing.T, ra io.ReaderAt, want []byte) {
	t.Helper()
	const workers = 8
	const iters = 200
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			for i := range iters {
				off := int64((w*131 + i*257) % (len(want) - len(buf)))
				n, err := ra.ReadAt(buf, off)
				require.NoError(t, err)
				require.Equal(t, len(buf), n)
				require.Equal(t, want[off:off+int64(n)], buf[:n])
			}
		}()
	}
	wg.Wait()
}

// modes drives the parametrised tests across both backings the
// platform offers: the default *file (fd-backed *os.File.ReadAt)
// and, on darwin/linux when WithMmap is passed, the *mmapFile.
// On platforms without mmap support the WithMmap row still runs
// but exercises the fd backing (WithMmap is best-effort).
var modes = []struct {
	name string
	opts []Option
}{
	{name: "fd"},
	{name: "mmap", opts: []Option{WithMmap()}},
}

func TestBoundOSConcurrentReadAt(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			want := writePattern(t, filepath.Join(dir, "data"), 64*1024)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			runConcurrentReadAt(t, f, want)
		})
	}
}

func TestRootOSConcurrentReadAt(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			want := writePattern(t, filepath.Join(dir, "data"), 64*1024)

			root, err := os.OpenRoot(dir)
			require.NoError(t, err)
			t.Cleanup(func() { _ = root.Close() })

			fs, err := FromRoot(root, m.opts...)
			require.NoError(t, err)

			f, err := fs.Open("data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			runConcurrentReadAt(t, f, want)
		})
	}
}

func TestOpenCloseSemantics(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writePattern(t, filepath.Join(dir, "data"), 4096)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("data")
			require.NoError(t, err)

			buf := make([]byte, 16)
			_, err = f.ReadAt(buf, 0)
			require.NoError(t, err)

			require.NoError(t, f.Close())
			_, err = f.ReadAt(buf, 0)
			require.ErrorIs(t, err, os.ErrClosed)
		})
	}
}

// TestOpenEmptyFile pins the empty-file path on both backings. With
// WithMmap on darwin/linux the mmap path's zero-size guard falls back
// to the regular *file wrapper; without it (or off-platform) it goes
// to *file directly. Both must return (0, io.EOF) at offset 0.
func TestOpenEmptyFile(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "empty"), nil, 0o600))

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("empty")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			buf := make([]byte, 8)
			n, err := f.ReadAt(buf, 0)
			require.Equal(t, 0, n)
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

// TestOpenReadSeekCursor exercises the mmap-backed file's Read+Seek
// cursor against the regular *file's equivalent semantics. Both
// backings must advance the cursor on Read, support SeekStart,
// SeekCurrent, and SeekEnd, and return io.EOF when read past the end.
func TestOpenReadSeekCursor(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			want := writePattern(t, filepath.Join(dir, "data"), 1024)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			// Sequential read advances cursor.
			head := make([]byte, 32)
			n, err := f.Read(head)
			require.NoError(t, err)
			require.Equal(t, 32, n)
			require.Equal(t, want[:32], head)

			// Continued read picks up where the cursor left off.
			next := make([]byte, 32)
			n, err = f.Read(next)
			require.NoError(t, err)
			require.Equal(t, 32, n)
			require.Equal(t, want[32:64], next)

			// SeekStart rewinds.
			pos, err := f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			require.Equal(t, int64(0), pos)
			again := make([]byte, 32)
			_, err = f.Read(again)
			require.NoError(t, err)
			require.Equal(t, want[:32], again)

			// SeekEnd lands at len(data); Read returns EOF.
			pos, err = f.Seek(0, io.SeekEnd)
			require.NoError(t, err)
			require.Equal(t, int64(1024), pos)
			tail := make([]byte, 32)
			n, err = f.Read(tail)
			require.Equal(t, 0, n)
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

// TestOpenReadOnlyMmapErrorsOnWrite asserts the mmap-backed file
// rejects writes. On platforms where mmap is unavailable the test
// row falls back to *file, whose writes return the os.File-level
// error for an O_RDONLY handle (also a write rejection). Either way
// Write must return an error.
func TestOpenReadOnlyMmapErrorsOnWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePattern(t, filepath.Join(dir, "data"), 256)

	fs := newTestBoundOS(t, dir, WithMmap())
	f, err := fs.Open("data")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	_, err = f.Write([]byte("nope"))
	require.Error(t, err)

	_, err = f.WriteAt([]byte("nope"), 0)
	require.Error(t, err)

	require.Error(t, f.Truncate(0))
}

// TestOpenStatNameAcrossModes pins f.Stat().Name() to the basename of
// the opened path on both backings. *file inherits this from
// *os.File.Stat(); *mmapFile delegates to the same. Without this
// pin, a future change to either wrapper could surface the full
// display path through Stat and diverge silently between modes.
func TestOpenStatNameAcrossModes(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
			writePattern(t, filepath.Join(dir, "sub", "data"), 256)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("sub/data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			info, err := f.Stat()
			require.NoError(t, err)
			require.Equal(t, "data", info.Name())
		})
	}
}

// TestOpenZeroLengthReadAtEOF pins that a Read/ReadAt with an empty
// buffer at EOF returns (0, nil) rather than (0, io.EOF), matching
// the fd backing's *os.File.Read behaviour. mmap's natural
// implementation (a bytes-style EOF check before the empty-buffer
// fast path) would otherwise diverge.
func TestOpenZeroLengthReadAtEOF(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writePattern(t, filepath.Join(dir, "data"), 8)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			// Advance the cursor to EOF, then issue a zero-length Read.
			_, err = f.Seek(0, io.SeekEnd)
			require.NoError(t, err)
			n, err := f.Read(nil)
			require.Equal(t, 0, n)
			require.NoError(t, err)

			// Same for ReadAt at an EOF-or-past offset.
			n, err = f.ReadAt(nil, 8)
			require.Equal(t, 0, n)
			require.NoError(t, err)
		})
	}
}

// TestOpenConcurrentReadCursorSafety asserts that concurrent Read
// calls on the same handle don't race on the cursor. Read is
// serialised internally; this test exists so a future change that
// drops the lock surfaces under -race.
func TestOpenConcurrentReadCursorSafety(t *testing.T) {
	t.Parallel()

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writePattern(t, filepath.Join(dir, "data"), 64*1024)

			fs := newTestBoundOS(t, dir, m.opts...)
			f, err := fs.Open("data")
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			const workers = 8
			const iters = 50
			var wg sync.WaitGroup
			wg.Add(workers)
			for range workers {
				go func() {
					defer wg.Done()
					buf := make([]byte, 256)
					for range iters {
						_, _ = f.Read(buf)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// TestOpenBackingSelection asserts that WithMmap actually flips the
// concrete backing on darwin/linux. On other platforms both modes
// return *file because mmap is unavailable.
func TestOpenBackingSelection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePattern(t, filepath.Join(dir, "data"), 4096)

	t.Run("default-is-fd", func(t *testing.T) {
		t.Parallel()
		fs := newTestBoundOS(t, dir)
		f, err := fs.Open("data")
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		_, ok := f.(*file)
		require.True(t, ok, "default backing should be *file, got %T", f)
	})

	t.Run("with-mmap-uses-mmap-when-available", func(t *testing.T) {
		t.Parallel()
		fs := newTestBoundOS(t, dir, WithMmap())
		f, err := fs.Open("data")
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		assertMmapBackingWhenAvailable(t, f)
	})

	t.Run("with-mmap-write-mode-still-uses-fd", func(t *testing.T) {
		t.Parallel()
		fs := newTestBoundOS(t, dir, WithMmap())
		f, err := fs.OpenFile("data", os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		_, ok := f.(*file)
		require.True(t, ok, "write-mode opens should bypass mmap, got %T", f)
	})
}

// Ensure *file satisfies billy.File at compile-time so type
// assertions in tests are guaranteed valid.
var _ billy.File = (*file)(nil)

func TestMmapFilesystemAdvertisesCapability(t *testing.T) {
	t.Parallel()

	osRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = osRoot.Close() })

	root, err := FromRoot(osRoot, WithMmap())
	require.NoError(t, err)
	require.NotZero(t, root.Capabilities()&billy.MmapCapability)

	plain, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = plain.Close() })

	plainFS, err := FromRoot(plain)
	require.NoError(t, err)
	require.Zero(t, plainFS.Capabilities()&billy.MmapCapability)
}
