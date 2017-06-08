// +build !windows

package osfs

import (
	"os"

	"gopkg.in/src-d/go-billy.v2"
)

// Stat returns the FileInfo structure describing file.
func (fs *OS) Stat(filename string) (billy.FileInfo, error) {
	fullpath := fs.absolutize(filename)
	return os.Stat(fullpath)
}
