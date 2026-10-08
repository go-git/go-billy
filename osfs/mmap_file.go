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
// via an RWMutex so munmap cannot run while a Read or ReadAt is in
// flight. Bytes hands out the mapping itself, which the lock cannot
// protect: Close or the GC cleanup unmaps it even while a caller
// still reads the returned slice. Write/WriteAt/Truncate return
// [os.ErrPermission] — the file is read-only by construction. The
// underlying descriptor is closed as soon as the mapping exists.
type mmapFile struct {
	data []byte
	name string
	// info is captured at open. The descriptor is closed once the
	// mapping exists, so Stat cannot fstat it later.
	info os.FileInfo

	mu     sync.RWMutex
	cursor int64
	closed bool

	cleanup runtime.Cleanup
}

// newMmapFile maps f read-only and returns an [*mmapFile]. Once the
// mapping is established f is closed: POSIX keeps a mapping valid after
// its descriptor is closed, so an mmap-backed file holds no descriptor.
//
// If mmap is unavailable for this particular file (zero size, size
// beyond platform int, mmap rejected by the kernel for pipes/devices
// etc.) the function returns [errMmapUnavailable] without closing f
// so the caller can fall back to a regular [*file] wrapper. Zero size
// includes procfs files, which report size 0 but have content.
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

	// The mapping is established and is what callers read from; a
	// close error on the now-unneeded descriptor is not actionable.
	_ = f.Close()

	m := &mmapFile{data: data, name: name, info: info}

	// Unmap when the mmapFile is garbage collected without Close.
	m.cleanup = runtime.AddCleanup(m, func(data []byte) {
		_ = unix.Munmap(data)
	}, data)

	return m, nil
}

func (m *mmapFile) Name() string { return m.name }

// Stat returns the FileInfo captured at open. Its Name() is the
// basename, matching the fd-backed *file. Size is exact because the
// mapping cannot grow; ModTime is the file's modification time as of
// open and may be stale.
func (m *mmapFile) Stat() (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, &os.PathError{Op: "stat", Path: m.name, Err: os.ErrClosed}
	}
	return m.info, nil
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

// Bytes implements [billy.BytesFile], returning the mapping itself. ok is
// false once the file has been closed.
//
// Writing to the read-only mapping, or reading past the end of the
// underlying file after it is truncated, crashes the process
// (SIGSEGV/SIGBUS) rather than returning an error. Using the slice after
// Close, or after an unreachable File is unmapped by its GC cleanup, is
// undefined behaviour: it may crash, or it may silently read another
// file's data once a later mapping reuses the address. Holding the slice
// does not keep the File alive, and the File can be collected after its
// last use while still in scope: call Close after the last use of the
// slice (for example with defer), or call runtime.KeepAlive on the File
// after it.
func (m *mmapFile) Bytes() (data []byte, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, false
	}
	return m.data, true
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

	err := unix.Munmap(m.data)
	m.data = nil
	return err
}
