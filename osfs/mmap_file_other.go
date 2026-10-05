//go:build !darwin && !linux && !js

package osfs

import "os"

// mmapFile is a stub on platforms without mmap support. The
// real implementation lives in mmap_file.go (darwin || linux);
// here the type exists solely so the wrapOpenedFile return
// path that constructs it still type-checks. newMmapFile
// always returns [errMmapUnavailable], so a value of this type
// is never actually constructed at runtime — the method bodies
// below are unreachable.
type mmapFile struct{}

func newMmapFile(_ *os.File, _ string) (*mmapFile, error) {
	return nil, errMmapUnavailable
}

func (m *mmapFile) Name() string                                  { return "" }
func (m *mmapFile) Stat() (os.FileInfo, error)                    { return nil, os.ErrInvalid }
func (m *mmapFile) Read(p []byte) (int, error)                    { return 0, os.ErrInvalid }
func (m *mmapFile) ReadAt(p []byte, off int64) (int, error)       { return 0, os.ErrInvalid }
func (m *mmapFile) Write(p []byte) (int, error)                   { return 0, os.ErrInvalid }
func (m *mmapFile) WriteAt(p []byte, off int64) (int, error)      { return 0, os.ErrInvalid }
func (m *mmapFile) Seek(offset int64, whence int) (int64, error)  { return 0, os.ErrInvalid }
func (m *mmapFile) Truncate(size int64) error                     { return os.ErrInvalid }
func (m *mmapFile) Close() error                                  { return os.ErrInvalid }
func (m *mmapFile) Bytes() []byte                                 { return nil }
func (m *mmapFile) Slice(off, length int64) []byte                { return nil }
