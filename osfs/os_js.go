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

	if o.Type == BoundOSFS {
		return newBoundOS(baseDir, o.deduplicatePath)
	}

	return newChrootOS(baseDir)
}

func newChrootOS(baseDir string) billy.Filesystem {
	return chroot.New(Default, Default.Join("/", baseDir))
}

// BoundOS is a fs implementation based on the js/wasm in-memory filesystem
// which is bound to a base dir.
type BoundOS struct {
	billy.Filesystem
	baseDir         string
	deduplicatePath bool
}

func newBoundOS(d string, deduplicatePath bool) billy.Filesystem {
	baseDir := globalMemFs.Join("/", d)
	return &BoundOS{
		Filesystem:      chroot.New(globalMemFs, baseDir),
		baseDir:         baseDir,
		deduplicatePath: deduplicatePath,
	}
}

// Chroot returns a new BoundOS filesystem, with the base dir set to the
// result of joining the provided path with the underlying base dir.
func (fs *BoundOS) Chroot(path string) (billy.Filesystem, error) {
	joined, err := fs.abs(path)
	if err != nil {
		return nil, err
	}
	return New(joined, WithBoundOS(), WithDeduplicatePath(fs.deduplicatePath)), nil
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

// WithChrootOS returns the option of using a Chroot filesystem OS.
func WithChrootOS() Option {
	return func(o *options) {
		o.Type = ChrootOSFS
	}
}

// WithDeduplicatePath toggles the deduplication of the base dir in the path.
// This option is accepted for API parity with non-js builds and has no effect
// on the js/wasm in-memory implementation.
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
