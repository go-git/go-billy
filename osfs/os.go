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
var Default = &ChrootOS{}

// New returns a new OS filesystem.
func New(baseDir string, opts ...Option) billy.Filesystem {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if o.Type == BoundOSFS {
		return newBoundOS(baseDir)
	}

	return newChrootOS(baseDir)
}

func tempFile(dir, prefix string) (billy.File, error) {
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, nil
}

func openFile(fn string, flag int, perm fs.FileMode, createDir func(string) error) (billy.File, error) {
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
	return &file{File: f}, err
}

// file is a wrapper for an os.File which adds support for file locking.
type file struct {
	*os.File
	m sync.Mutex
}

func (f *file) WriteTo(w io.Writer) error {
	_, err := f.File.WriteTo(w)
	return err
}
