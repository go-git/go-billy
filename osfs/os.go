//go:build !js

// Package osfs provides a billy filesystem for the OS.
package osfs

import (
	"fmt"
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
var Default = newBoundOS(string(os.PathSeparator), true)

// New returns a new OS filesystem.
// By default paths are deduplicated, but still enforced
// under baseDir. For more info refer to WithDeduplicatePath.
func New(baseDir string, opts ...Option) billy.Filesystem {
	o := &options{
		deduplicatePath: true,
	}
	for _, opt := range opts {
		opt(o)
	}

	return newBoundOS(baseDir, o.deduplicatePath)
}

// WithBoundOS returns the option of using a Bound filesystem OS.
func WithBoundOS() Option {
	return func(o *options) {
		o.Type = BoundOSFS
	}
}

// WithDeduplicatePath toggles the deduplication of the base dir in the path.
// This occurs when absolute links are being used.
// Assuming base dir /base/dir and an absolute symlink /base/dir/target:
//
// With DeduplicatePath (default): /base/dir/target
// Without DeduplicatePath: /base/dir/base/dir/target
//
// This option is only used by the BoundOS OS type.
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

func tempFile(dir, prefix, name string) (billy.File, error) {
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &file{File: f, name: name}, nil
}

func openFile(fn, name string, flag int, perm fs.FileMode, createDir func(string) error) (billy.File, error) {
	if flag&os.O_CREATE != 0 {
		if createDir == nil {
			return nil, fmt.Errorf("createDir func cannot be nil if file needs to be opened in create mode")
		}
		if err := createDir(fn); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(fn, flag, perm)
	if err != nil {
		return nil, err
	}
	return &file{File: f, name: name}, err
}

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
