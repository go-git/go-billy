//go:build !windows && !wasip1 && !js && !wasp
// +build !windows,!wasip1,!js,!wasp

package test

import (
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
)

var (
	customMode            os.FileMode = 0o755
	expectedSymlinkTarget             = "/dir/file"
)

func allFS(tempDir func() string) []billy.Filesystem {
	return []billy.Filesystem{
		osfs.New(tempDir(), osfs.WithChrootOS()),
		memfs.New(),
	}
}
