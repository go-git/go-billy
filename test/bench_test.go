package test

import (
	"crypto/rand"
	"fmt"
	"io"
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
	contentSize = 1024 * 10
	bufSize     = 1024
)

type test struct {
	name    string
	fn      string
	sut     billy.Filesystem
	openF   func(billy.Filesystem) func(string) (io.ReadSeekCloser, error)
	createF func(billy.Filesystem, string) (io.WriteCloser, error)
}

func BenchmarkCompare(b *testing.B) {
	tests := []test{
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
			sut:     memfs.New(memfs.WithoutMutex()),
			openF:   billyOpen,
			createF: billyCreate,
		},
		{
			name:    "memfs_mutex",
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
		b.Run(tc.name+"_open", open(tc.fn, tc.openF(tc.sut)))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_read", read(tc.fn, tc.openF(tc.sut)))
	}

	for _, tc := range tests {
		b.Run(tc.name+"_create", create(tc.sut, tc.fn, tc.createF))
	}
}

func create(fs billy.Filesystem, n string, nf func(billy.Filesystem, string) (io.WriteCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fn := fmt.Sprintf("%s_%d", n, i)
			b.StartTimer()
			f, err := nf(fs, fn)
			b.StopTimer()

			require.NoError(b, err)
			assert.NotNil(b, f)

			err = f.Close()
			require.NoError(b, err)
		}
	}
}

func open(n string, of func(string) (io.ReadSeekCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, err := of(n)
			require.NoError(b, err)
			assert.NotNil(b, f)

			b.StopTimer()
			err = f.Close()
			require.NoError(b, err)
			b.StartTimer()
		}
	}
}

func read(n string, of func(string) (io.ReadSeekCloser, error)) func(b *testing.B) {
	return func(b *testing.B) {
		b.StopTimer()

		buf := make([]byte, 1024)
		f, err := of(n)
		require.NoError(b, err)
		assert.NotNil(b, f)

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_, err = f.Seek(0, io.SeekStart)
			require.NoError(b, err)

			for {
				n, err := f.Read(buf)
				if n == 0 {
					break
				}
				b.SetBytes(int64(n))
				require.NoError(b, err)
			}
		}

		err = f.Close()
		require.NoError(b, err)
	}
}

func prepFS(b *testing.B, f io.WriteCloser) {
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
