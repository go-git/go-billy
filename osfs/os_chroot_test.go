//go:build !wasm
// +build !wasm

package osfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ChrootOSSuite struct {
	test.FilesystemSuite
	path string
}

var _ = Suite(&ChrootOSSuite{})

func (s *ChrootOSSuite) SetUpTest(c *C) {
	s.path, _ = ioutil.TempDir(os.TempDir(), "go-billy-osfs-test")
	if runtime.GOOS == "plan9" {
		// On Plan 9, permission mode of newly created files
		// or directories are based on the permission mode of
		// the containing directory (see http://man.cat-v.org/plan_9/5/open).
		// Since TestOpenFileWithModes and TestStat creates files directly
		// in the temporary directory, we need to make it more permissive.
		c.Assert(os.Chmod(s.path, 0777), IsNil)
	}
	s.FilesystemSuite = test.NewFilesystemSuite(newChrootOS(s.path))
}

func (s *ChrootOSSuite) TearDownTest(c *C) {
	err := os.RemoveAll(s.path)
	c.Assert(err, IsNil)
}

func (s *ChrootOSSuite) TestOpenDoesNotCreateDir(c *C) {
	_, err := s.FS.Open("dir/non-existent")
	c.Assert(err, NotNil)

	_, err = os.Stat(filepath.Join(s.path, "dir"))
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *ChrootOSSuite) TestEmptyBaseChrootPreservesAbsolutePath(c *C) {
	dir := c.MkDir()
	c.Assert(os.Mkdir(filepath.Join(dir, ".git"), 0755), IsNil)

	fs, err := newChrootOS("").Chroot(dir)
	c.Assert(err, IsNil)
	c.Assert(fs.Root(), Equals, dir)

	fi, err := fs.Stat(".git")
	c.Assert(err, IsNil)
	c.Assert(fi.IsDir(), Equals, true)
}

func (s *ChrootOSSuite) TestSymlinkedRoot(c *C) {
	root := c.MkDir()
	realRoot := filepath.Join(root, "real")
	linkRoot := filepath.Join(root, "link")

	c.Assert(os.Mkdir(realRoot, 0755), IsNil)
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		c.Skip("symlink creation is not supported")
	}

	fs := newChrootOS(linkRoot)

	fi, err := fs.Stat("/")
	c.Assert(err, IsNil)
	c.Assert(fi.IsDir(), Equals, true)

	_, err = fs.Stat(".git")
	c.Assert(os.IsNotExist(err), Equals, true)
	c.Assert(err, Not(Equals), billy.ErrCrossedBoundary)
}

func (s *ChrootOSSuite) TestCapabilities(c *C) {
	_, ok := s.FS.(billy.Capable)
	c.Assert(ok, Equals, true)

	caps := billy.Capabilities(s.FS)
	c.Assert(caps, Equals, billy.AllCapabilities)
}
