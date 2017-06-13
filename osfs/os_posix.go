// +build !windows

package osfs

import (
	"os"
)

// Stat returns the FileInfo structure describing file.
func (fs *OS) Stat(filename string) (os.FileInfo, error) {
	fullpath, err := fs.absolutize(filename)
	if err != nil {
		return nil, err
	}

	return os.Stat(fullpath)
}
