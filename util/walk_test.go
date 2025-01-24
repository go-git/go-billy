package util_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var targetSubfolder = filepath.FromSlash("path/to/some/subfolder")

func TestWalkCanSkipTopDirectory(t *testing.T) {
	filesystem := memfs.New()
	err := util.Walk(filesystem, "/root/that/does/not/exist", func(path string, info os.FileInfo, err error) error { return filepath.SkipDir })

	assert.NoError(t, err)
}

func TestWalkReturnsAnErrorWhenRootDoesNotExist(t *testing.T) {
	filesystem := memfs.New()
	err := util.Walk(filesystem, "/root/that/does/not/exist", func(path string, info os.FileInfo, err error) error { return err })

	assert.Error(t, err)
}

func TestWalkOnPlainFile(t *testing.T) {
	filesystem := memfs.New()
	createFile(t, filesystem, "./README.md")
	discoveredPaths := []string{}

	err := util.Walk(filesystem, "./README.md", func(path string, info os.FileInfo, err error) error {
		discoveredPaths = append(discoveredPaths, path)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"./README.md"}, discoveredPaths)
}

func TestWalkOnExistingFolder(t *testing.T) {
	filesystem := memfs.New()
	createFile(t, filesystem, "path/to/some/subfolder/that/contain/file")
	createFile(t, filesystem, "path/to/some/file")
	discoveredPaths := []string{}
	err := util.Walk(filesystem, "path", func(path string, info os.FileInfo, err error) error {
		discoveredPaths = append(discoveredPaths, path)
		return nil
	})
	assert.NoError(t, err)

	assert.Contains(t, discoveredPaths, "path")
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/file"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain/file"))
}

func TestWalkCanSkipFolder(t *testing.T) {
	filesystem := memfs.New()
	createFile(t, filesystem, "path/to/some/subfolder/that/contain/file")
	createFile(t, filesystem, "path/to/some/file")
	discoveredPaths := []string{}
	err := util.Walk(filesystem, "path", func(path string, info os.FileInfo, err error) error {
		discoveredPaths = append(discoveredPaths, path)
		if path == targetSubfolder {
			return filepath.SkipDir
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Contains(t, discoveredPaths, "path")
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/file"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain/file"))
}

func TestWalkStopsOnError(t *testing.T) {
	filesystem := memfs.New()
	createFile(t, filesystem, "path/to/some/subfolder/that/contain/file")
	createFile(t, filesystem, "path/to/some/file")
	discoveredPaths := []string{}
	err := util.Walk(filesystem, "path", func(path string, info os.FileInfo, err error) error {
		discoveredPaths = append(discoveredPaths, path)
		if path == targetSubfolder {
			return errors.New("uncaught error")
		}
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, discoveredPaths, "path")
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/file"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain/file"))
}

func TestWalkForwardsStatErrors(t *testing.T) {
	memFilesystem := memfs.New()
	filesystem := &fnFs{
		Filesystem: memFilesystem,
		lstat: func(path string) (os.FileInfo, error) {
			if path == targetSubfolder {
				return nil, errors.New("uncaught error")
			}
			return memFilesystem.Lstat(path)
		},
	}

	createFile(t, filesystem, "path/to/some/subfolder/that/contain/file")
	createFile(t, filesystem, "path/to/some/file")
	discoveredPaths := []string{}
	err := util.Walk(filesystem, "path", func(path string, info os.FileInfo, err error) error {
		discoveredPaths = append(discoveredPaths, path)
		if path == targetSubfolder {
			assert.Error(t, err)
		}
		return err
	})
	assert.Error(t, err)
	assert.Contains(t, discoveredPaths, "path")
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/file"))
	assert.Contains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain"))
	assert.NotContains(t, discoveredPaths, filepath.FromSlash("path/to/some/subfolder/that/contain/file"))
}

func createFile(t *testing.T, filesystem billy.Filesystem, path string) {
	fd, err := filesystem.Create(path)

	require.NoError(t, err)
	fd.Close()
}

type fnFs struct {
	billy.Filesystem
	lstat func(path string) (os.FileInfo, error)
}

func (f *fnFs) Lstat(path string) (os.FileInfo, error) {
	if f.lstat != nil {
		return f.lstat(path)
	}
	return nil, errors.New("not implemented")
}
