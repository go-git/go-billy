//go:build js

package osfs

import (
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/helper/chroot"
	"github.com/go-git/go-billy/v6/memfs"
)

// globalMemFs is the global memory fs
var globalMemFs = memfs.New()

// Default Filesystem representing the root of in-memory filesystem for a
// js/wasm environment.
var Default = memfs.New()

// New returns a new OS filesystem rooted at baseDir, backed by the
// js/wasm in-memory filesystem.
func New(baseDir string, opts ...Option) billy.Filesystem {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return newBoundOS(baseDir)
}

// BoundOS is a fs implementation based on the js/wasm in-memory filesystem
// which is bound to a base dir.
type BoundOS struct {
	billy.Filesystem
	baseDir string
}

func newBoundOS(d string) billy.Filesystem {
	baseDir := globalMemFs.Join("/", d)
	return &BoundOS{
		Filesystem: chroot.New(globalMemFs, baseDir),
		baseDir:    baseDir,
	}
}

// Capabilities implements the billy.Capable interface, delegating to the
// underlying in-memory filesystem.
func (fs *BoundOS) Capabilities() billy.Capability {
	return billy.Capabilities(fs.Filesystem)
}

// Chroot returns a new BoundOS filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	joined, err := fs.abs(path)
	if err != nil {
		return nil, err
	}
	return New(joined, WithBoundOS()), nil
}

// Root returns the current base dir of the billy.Filesystem.
func (fs *BoundOS) Root() string {
	return fs.baseDir
}

func (fs *BoundOS) abs(filename string) (string, error) {
	if filename == "." || filename == "" {
		return fs.baseDir, nil
	}

	filename = filepath.ToSlash(filename)
	filename = filepath.Clean(filename)
	if strings.HasPrefix(filename, "..") {
		return "", billy.ErrCrossedBoundary
	}

	filename = strings.TrimPrefix(filename, string(filepath.Separator))
	return filepath.Clean(filepath.Join(fs.baseDir, filepath.FromSlash(filename))), nil
}

// WithBoundOS returns the option of using a Bound filesystem OS.
func WithBoundOS() Option {
	return func(o *options) {
		o.Type = BoundOSFS
	}
}

type Option func(*options)

type options struct {
	Type
}

type Type int

const (
	BoundOSFS Type = iota
)
