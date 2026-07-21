//go:build !js && !wasm && !wasip1

package test

import (
	"crypto/rand"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/require"
)

// BenchmarkCopy measures util.Copy throughput across osfs and memfs
// source/destination pairs. osfs to osfs may use Linux fd-based copy acceleration;
// pairs involving memfs use Open+Create+io.Copy.
func BenchmarkCopy(b *testing.B) {
	content := make([]byte, contentSize)
	_, err := rand.Read(content)
	require.NoError(b, err)

	srcDir := b.TempDir()
	dstDir := b.TempDir()

	cases := []struct {
		name string
		src  billy.Filesystem
		dst  billy.Filesystem
	}{
		{name: "osfs_to_osfs", src: osfs.New(srcDir, osfs.WithBoundOS()), dst: osfs.New(dstDir, osfs.WithBoundOS())},
		{name: "memfs_to_memfs", src: memfs.New(), dst: memfs.New()},
		{name: "memfs_to_osfs", src: memfs.New(), dst: osfs.New(dstDir, osfs.WithBoundOS())},
		{name: "osfs_to_memfs", src: osfs.New(srcDir, osfs.WithBoundOS()), dst: memfs.New()},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(contentSize)

			f, err := tc.src.OpenFile("src", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			require.NoError(b, err)
			_, err = f.Write(content)
			require.NoError(b, err)
			require.NoError(b, f.Close())

			i := 0
			for b.Loop() {
				dst := "dst_" + strconv.Itoa(i)
				i++

				err := util.Copy(tc.src, "src", tc.dst, dst)

				b.StopTimer()
				require.NoError(b, err)
				if i == 1 {
					r, err := tc.dst.Open(dst)
					require.NoError(b, err)
					got, err := io.ReadAll(r)
					_ = r.Close()
					require.NoError(b, err)
					require.Equal(b, content, got)
				}
				_ = tc.dst.Remove(dst)
				b.StartTimer()
			}
		})
	}
}
