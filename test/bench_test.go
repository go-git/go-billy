package test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fn          = "test"
	contentSize = 1024 * 1024 // 1 MB – large enough for stable throughput numbers
	bufSize     = 32 * 1024   // 32 KB – realistic block size for sequential I/O
)

type btest struct {
	name    string
	fn      string
	sut     billy.Filesystem
	openF   func(billy.Filesystem) func(string) (io.ReadSeekCloser, error)
	createF func(billy.Filesystem, string) (io.WriteCloser, error)
}

func BenchmarkCompare(b *testing.B) {
	b.ReportAllocs()

	tests := []btest{
		{
			// provide baseline comparison against direct use of os.
			name:    "stdlib",
			fn:      filepath.Join(b.TempDir(), fn),
			openF:   stdlibOpen,
			createF: stdlibCreate,
		},
		{
			name:    "osfs.chrootOS",
			fn:      fn,
			sut:     osfs.New(b.TempDir(), osfs.WithChrootOS()),
			openF:   billyOpen,
			createF: billyCreate,
		},
		{
			name:    "osfs.boundOS",
			fn:      fn,
			sut:     osfs.New(b.TempDir(), osfs.WithBoundOS()),
			openF:   billyOpen,
			createF: billyCreate,
		},
		{
			name:    "memfs",
			fn:      fn,
			sut:     memfs.New(),
			openF:   billyOpen,
			createF: billyCreate,
		},
	}

	for _, tc := range tests {
		f, err := tc.createF(tc.sut, tc.fn)
		require.NoError(b, err)
		assert.NotNil(b, f)

		prepFS(b, f)
		b.Run(tc.name+"_open", benchOpen(tc.fn, tc.openF(tc.sut)))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_read", benchRead(tc.fn, tc.openF(tc.sut)))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_write", benchWrite(tc.sut, tc.fn, tc.createF))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_create", benchCreate(tc.sut, tc.fn, tc.createF))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_stat", benchStat(tc.fn, tc.sut))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_rename", benchRename(tc.sut))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_remove", benchRemove(tc.sut))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_mkdirall", benchMkdirAll(tc.sut))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_tempfile", benchTempFile(tc.sut))
	}
}

func benchCreate(filesystem billy.Filesystem, n string, nf func(billy.Filesystem, string) (io.WriteCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			name := fmt.Sprintf("%s_create_%d", n, i)
			i++

			f, err := nf(filesystem, name)

			require.NoError(b, err)
			assert.NotNil(b, f)

			b.StopTimer()
			err = f.Close()
			require.NoError(b, err)
			b.StartTimer()
		}
	}
}

func benchWrite(filesystem billy.Filesystem, n string, nf func(billy.Filesystem, string) (io.WriteCloser, error)) func(b *testing.B) {
	content := make([]byte, contentSize)
	_, err := rand.Read(content)
	if err != nil {
		panic(err)
	}

	return func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(contentSize)

		i := 0
		for b.Loop() {
			name := fmt.Sprintf("%s_write_%d", n, i)
			i++

			f, err := nf(filesystem, name)
			require.NoError(b, err)

			buf := content
			for len(buf) > 0 {
				nw, err := f.Write(buf[:min(bufSize, len(buf))])
				require.NoError(b, err)
				buf = buf[nw:]
			}

			b.StopTimer()
			require.NoError(b, f.Close())
			b.StartTimer()
		}
	}
}

func benchOpen(n string, of func(string) (io.ReadSeekCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			f, err := of(n)

			b.StopTimer()
			require.NoError(b, err)
			assert.NotNil(b, f)
			err = f.Close()
			require.NoError(b, err)
			b.StartTimer()
		}
	}
}

func benchRead(n string, of func(string) (io.ReadSeekCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.StopTimer()

		buf := make([]byte, bufSize)
		f, err := of(n)
		require.NoError(b, err)
		assert.NotNil(b, f)

		b.StartTimer()
		for b.Loop() {
			_, err = f.Seek(0, io.SeekStart)
			require.NoError(b, err)

			for {
				n, err := f.Read(buf)
				if n > 0 {
					b.SetBytes(int64(n))
				}
				if errors.Is(err, io.EOF) {
					break
				}
				require.NoError(b, err)
			}
		}

		b.StopTimer()
		err = f.Close()
		require.NoError(b, err)
	}
}

func benchStat(n string, filesystem billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		if filesystem == nil {
			b.Skip("stat benchmark not supported for stdlib baseline")
		}
		b.ReportAllocs()
		for b.Loop() {
			fi, err := filesystem.Stat(n)

			b.StopTimer()
			require.NoError(b, err)
			assert.NotNil(b, fi)
			b.StartTimer()
		}
	}
}

func benchRename(filesystem billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		if filesystem == nil {
			b.Skip("rename benchmark not supported for stdlib baseline")
		}
		b.ReportAllocs()

		i := 0
		src := fmt.Sprintf("rename_src_%d", i)
		i++

		// Seed the first source file.
		b.StopTimer()
		f, err := filesystem.Create(src)
		require.NoError(b, err)
		require.NoError(b, f.Close())
		b.StartTimer()

		for b.Loop() {
			dst := fmt.Sprintf("rename_dst_%d", i)
			i++

			err := filesystem.Rename(src, dst)

			b.StopTimer()
			require.NoError(b, err)
			src = dst
			b.StartTimer()
		}

		b.StopTimer()
		_ = filesystem.Remove(src)
	}
}

func benchRemove(filesystem billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		if filesystem == nil {
			b.Skip("remove benchmark not supported for stdlib baseline")
		}
		b.ReportAllocs()

		i := 0
		for b.Loop() {
			name := fmt.Sprintf("remove_%d", i)
			i++

			b.StopTimer()
			f, err := filesystem.Create(name)
			require.NoError(b, err)
			require.NoError(b, f.Close())
			b.StartTimer()

			err = filesystem.Remove(name)

			b.StopTimer()
			require.NoError(b, err)
			b.StartTimer()
		}
	}
}

func benchMkdirAll(filesystem billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		if filesystem == nil {
			b.Skip("mkdirall benchmark not supported for stdlib baseline")
		}
		b.ReportAllocs()

		i := 0
		for b.Loop() {
			dir := fmt.Sprintf("benchdir/sub/leaf_%d", i)
			i++

			err := filesystem.MkdirAll(dir, fs.ModePerm)

			b.StopTimer()
			require.NoError(b, err)
			b.StartTimer()
		}
	}
}

func benchTempFile(filesystem billy.Filesystem) func(b *testing.B) {
	return func(b *testing.B) {
		if filesystem == nil {
			b.Skip("tempfile benchmark not supported for stdlib baseline")
		}
		b.ReportAllocs()

		for b.Loop() {
			f, err := filesystem.TempFile("", "bench")

			b.StopTimer()
			require.NoError(b, err)
			assert.NotNil(b, f)
			_ = filesystem.Remove(f.Name())
			b.StartTimer()
		}
	}
}

func prepFS(b *testing.B, f io.WriteCloser) {
	b.Helper()
	defer f.Close()

	content := make([]byte, contentSize)
	_, err := rand.Read(content)
	require.NoError(b, err, "failed to generate random content")

	_, err = f.Write(content)
	require.NoError(b, err, "failed to write test file")
}

func stdlibOpen(_ billy.Filesystem) func(n string) (io.ReadSeekCloser, error) {
	return func(n string) (io.ReadSeekCloser, error) {
		return os.OpenFile(n, os.O_RDONLY, 0o444)
	}
}

func billyOpen(fs billy.Filesystem) func(n string) (io.ReadSeekCloser, error) {
	return func(n string) (io.ReadSeekCloser, error) {
		return fs.Open(n)
	}
}

func stdlibCreate(_ billy.Filesystem, n string) (io.WriteCloser, error) {
	return os.Create(n)
}

func billyCreate(fs billy.Filesystem, n string) (io.WriteCloser, error) {
	return fs.Create(n)
}
