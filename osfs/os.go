//go:build !js

// Package osfs provides a billy filesystem for the OS.
package osfs

import (
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
var Default = newBoundOS(string(os.PathSeparator))

// New returns a new OS filesystem rooted at baseDir.
//
// The returned filesystem is always a [BoundOS]: containment is enforced
// via [os.Root], opened and closed per operation. For better performance
// with caller-managed lifecycle, use [FromRoot] instead.
//
// All [Option] values are accepted for API compatibility but have no
// effect on the returned implementation.
func New(baseDir string, opts ...Option) billy.Filesystem {
	o := &options{
		deduplicatePath: true,
	}
	for _, opt := range opts {
		opt(o)
	}

	return newBoundOS(baseDir)
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

// WithDeduplicatePath toggles the deduplication of the base dir in the path.
//
// BoundOS now relies on os.Root for path containment, so this option is kept
// for API compatibility and has no effect.
func WithDeduplicatePath(enabled bool) Option {
	return func(o *options) {
		o.deduplicatePath = enabled
	}
}

type options struct {
	Type
	deduplicatePath bool
}

type Type int

const (
	ChrootOSFS Type = iota
	BoundOSFS
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
