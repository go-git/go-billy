//go:build wasip1 || wasm || js

package test

import (
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

var (
	customMode            os.FileMode = 0o600
	expectedSymlinkTarget             = "/dir/file"
)

func allFS(_ func() string) []billy.Filesystem {
	return []billy.Filesystem{
		memfs.New(),
	}
}
