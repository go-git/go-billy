//go:build !js

// Package osfs provides a billy filesystem for the OS.
package osfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/go-git/go-billy/v6"
)

const (
	defaultDirectoryMode = 0o777
	defaultCreateMode    = 0o666
)

// Default Filesystem representing the root of the os filesystem.
var Default billy.Filesystem = newBoundOS(string(os.PathSeparator))

// New returns a new OS filesystem rooted at baseDir.
//
// The returned filesystem is always a [BoundOS]: containment is enforced
// via [os.Root], opened and closed per operation. For better performance
// with caller-managed lifecycle, use [FromRoot] instead.
//
// [WithMmap] enables an mmap-backed implementation of [BoundOS.Open] on
// supported platforms; all other [Option] values are accepted for API
// compatibility but have no effect on the returned implementation.
func New(baseDir string, opts ...Option) billy.Filesystem {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	fs := newBoundOS(baseDir)
	fs.mmap = o.mmap
	return fs
}

// WithBoundOS selects the [BoundOS] implementation.
//
// [BoundOS] is the only OS-backed implementation returned by [New], so this
// option is the default and is kept for API compatibility.
func WithBoundOS() Option {
	return func(o *options) {
		o.Type = BoundOSFS
	}
}

type options struct {
	Type
	mmap bool
}

// errMmapUnavailable is returned by newMmapFile when the file cannot
// be memory-mapped for benign reasons (zero size, size beyond
// platform int, mmap rejected by the kernel, platform without mmap
// support). Callers in osfs treat this as a fall-through signal and
// return the regular *file wrapper instead.
var errMmapUnavailable = errors.New("osfs: mmap unavailable for this file")

// Type identifies an osfs implementation.
type Type int

const (
	// BoundOSFS selects the [BoundOS] implementation.
	BoundOSFS Type = iota
)

// file is a wrapper for an os.File which adds support for file locking.
type file struct {
	*os.File
	name string
	m    sync.Mutex
}

func (f *file) Name() string {
	if f.name != "" {
		return f.name
	}
	return f.File.Name()
}

func (f *file) WriteTo(w io.Writer) error {
	_, err := f.File.WriteTo(w)
	return err
}

type fileInfo struct {
	fs.FileInfo
	name string
}

func (fi fileInfo) Name() string {
	return fi.name
}
