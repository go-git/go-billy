package osfs

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/memfs"
)

// globalMemFs is the global memory fs
var globalMemFs = memfs.New()

const (
	defaultDirectoryMode = 0755
	defaultCreateMode    = 0666
)

// OS is a filesystem shim for js.
type OS struct{}

// New returns a new OS filesystem.
func New(baseDir string) billy.Filesystem {
	return chroot.New(globalMemFs, baseDir)
}
