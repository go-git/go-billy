package memfs

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFileConcurrentReadAt asserts that concurrent ReadAt calls on a
// shared memfs file handle are safe under -race and return correct
// per-byte results. memfs is the in-memory billy backing that go-git
// and other consumers rely on for tests; the billy.File.ReadAt
// contract documents concurrent-safety as required, and this test
// pins memfs's compliance.
func TestFileConcurrentReadAt(t *testing.T) {
	t.Parallel()

	const size = 64 * 1024
	want := make([]byte, size)
	for i := range want {
		want[i] = byte(i % 251)
	}

	fs := New()
	f, err := fs.Create("data")
	require.NoError(t, err)
	_, err = f.Write(want)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	rf, err := fs.Open("data")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rf.Close() })

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
				n, err := rf.ReadAt(buf, off)
				require.NoError(t, err)
				require.Equal(t, len(buf), n)
				require.Equal(t, want[off:off+int64(n)], buf[:n])
			}
		}()
	}
	wg.Wait()
}
