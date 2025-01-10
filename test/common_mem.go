//go:build wasip1 || wasm || js

package test

import (
	"io/fs"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
)

var (
	customMode            fs.FileMode = 0o600
	expectedSymlinkTarget             = "/dir/file"
)

func allFS(_ func() string) []billy.Filesystem {
	return []billy.Filesystem{
		memfs.New(),
	}
}
