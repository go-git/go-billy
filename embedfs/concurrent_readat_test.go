package embedfs

import (
	"embed"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/concurrent.bin
var concurrentFS embed.FS

// TestFileConcurrentReadAt asserts that concurrent ReadAt calls on a
// shared embedfs file handle are safe under -race and return correct
// per-byte results. embedfs wraps bytes.Reader, which is already
// concurrent-safe for ReadAt; this test pins the billy.File.ReadAt
// contract so future changes to the wrapper cannot regress it.
func TestFileConcurrentReadAt(t *testing.T) {
	t.Parallel()

	want, err := concurrentFS.ReadFile("testdata/concurrent.bin")
	require.NoError(t, err)
	require.Equal(t, 4096, len(want))

	fs := New(&concurrentFS)
	f, err := fs.Open("testdata/concurrent.bin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	const workers = 8
	const iters = 200
	const bufLen = 64

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func() {
			defer wg.Done()
			buf := make([]byte, bufLen)
			for i := range iters {
				off := int64((w*131 + i*257) % (len(want) - bufLen))
				n, err := f.ReadAt(buf, off)
				require.NoError(t, err)
				require.Equal(t, bufLen, n)
				require.Equal(t, want[off:off+int64(n)], buf[:n])
			}
		}()
	}
	wg.Wait()
}
