package temporal

import (
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
)

// Temporal is a helper that implements billy.TempFile over any filesystem.
type Temporal struct {
	billy.Filesystem
	defaultDir string
}

// New creates a new filesystem wrapping up 'fs' the intercepts the calls to
// the TempFile method. The param defaultDir is used as default directory were
// the tempfiles are created.
func New(fs billy.Filesystem, defaultDir string) billy.Filesystem {
	return &Temporal{
		Filesystem: fs,
		defaultDir: defaultDir,
	}
}

func (h *Temporal) TempFile(dir, prefix string) (billy.File, error) {
	if dir == "" {
		dir = h.defaultDir
	}

	return util.TempFile(h.Filesystem, dir, prefix)
}
