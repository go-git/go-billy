//go:build darwin || linux

package osfs

import (
	"errors"
	"io"
	"math"
	"os"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

// mmapFile is a billy.File backed by a read-only memory map. It is
// returned from BoundOS/RootOS.OpenFile when the filesystem was
// constructed with [WithMmap] and the file is opened without write
// flags. Read and Seek track a real cursor over the mapped bytes,
// ReadAt is concurrent-safe (multiple goroutines may call it in
// parallel against the same handle) and serialised against Close
// via an RWMutex so munmap cannot run while a read is in flight.
// Write/WriteAt/Truncate return [os.ErrPermission] — the file is
// read-only by construction.
type mmapFile struct {
	f    *os.File
	data []byte
	name string

	mu     sync.RWMutex
	cursor int64
	closed bool

	cleanup runtime.Cleanup
}

// newMmapFile maps f read-only and returns an [*mmapFile] that owns
// f. On success the returned handle is responsible for closing the
// underlying [*os.File] via [(*mmapFile).Close].
//
// If mmap is unavailable for this particular file (zero size, size
// beyond platform int, mmap rejected by the kernel for pipes/devices
// etc.) the function returns [errMmapUnavailable] without closing f
// so the caller can fall back to a regular [*file] wrapper.
//
// Any other error (e.g. fstat failing) is propagated as-is and f is
// closed before returning — the caller must not use it.
func newMmapFile(f *os.File, name string) (*mmapFile, error) {
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	size := info.Size()
	if size <= 0 || size > int64(math.MaxInt) {
		// unix.Mmap rejects size 0, and 32-bit platforms can't
		// represent very large mappings as an int. Either case
		// is fine for the regular fd wrapper.
		return nil, errMmapUnavailable
	}

	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		// Many failure modes here are legitimate (pipes, devices,
		// FS quirks). Defer to the fd wrapper.
		return nil, errMmapUnavailable
	}

	m := &mmapFile{f: f, data: data, name: name}

	// unmap and close the file when the mmapFile is garbage collected and no
	// Close is called before.
	closeFunc := func(_ struct{}) {
		_ = unix.Munmap(data)
		_ = f.Close()
	}
	m.cleanup = runtime.AddCleanup(m, closeFunc, struct{}{})

	return m, nil
}

func (m *mmapFile) Name() string { return m.name }

// Stat returns the underlying *os.File's FileInfo unchanged so that
// f.Stat().Name() matches the basename returned by the fd-backed
// *file across both backings.
func (m *mmapFile) Stat() (os.FileInfo, error) {
	return m.f.Stat()
}

// Read implements [io.Reader]. It holds the write lock because it
// mutates the shared cursor; concurrent Read+Read would otherwise
// race on m.cursor even though both could read m.data under RLock.
// Random-access callers should use ReadAt, which is the parallel API.
func (m *mmapFile) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, os.ErrClosed
	}
	if m.cursor >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.cursor:])
	m.cursor += int64(n)
	return n, nil
}

func (m *mmapFile) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return 0, os.ErrClosed
	}
	if off < 0 {
		return 0, &os.PathError{Op: "readat", Path: m.name, Err: errors.New("negative offset")}
	}
	if off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *mmapFile) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, os.ErrClosed
	}
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = m.cursor + offset
	case io.SeekEnd:
		abs = int64(len(m.data)) + offset
	default:
		return 0, &os.PathError{Op: "seek", Path: m.name, Err: errors.New("invalid whence")}
	}
	if abs < 0 {
		return 0, &os.PathError{Op: "seek", Path: m.name, Err: errors.New("negative position")}
	}
	m.cursor = abs
	return abs, nil
}

func (m *mmapFile) Write(p []byte) (int, error) {
	return 0, &os.PathError{Op: "write", Path: m.name, Err: os.ErrPermission}
}

func (m *mmapFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, &os.PathError{Op: "writeat", Path: m.name, Err: os.ErrPermission}
}

func (m *mmapFile) Truncate(size int64) error {
	return &os.PathError{Op: "truncate", Path: m.name, Err: os.ErrPermission}
}

func (m *mmapFile) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return os.ErrClosed
	}
	m.closed = true
	m.cleanup.Stop()

	munmapErr := unix.Munmap(m.data)
	m.data = nil
	closeErr := m.f.Close()
	return errors.Join(munmapErr, closeErr)
}
