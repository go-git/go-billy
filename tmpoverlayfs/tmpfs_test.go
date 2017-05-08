package tmpfs

import (
	"io/ioutil"
	stdos "os"
	"testing"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/osfs"
	"gopkg.in/src-d/go-billy.v2/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type FilesystemSuite struct {
	test.FilesystemSuite
	cfs     billy.Filesystem
	path    string
	pathTmp string
}

var _ = Suite(&FilesystemSuite{})

func (s *FilesystemSuite) SetUpTest(c *C) {
	s.path, _ = ioutil.TempDir(stdos.TempDir(), "go-billy-tmpfs-test")
	fs := osfs.New(s.path)

	s.pathTmp, _ = ioutil.TempDir(stdos.TempDir(), "go-billy-tmpfs-test-tmp")
	fsTmp := osfs.New(s.pathTmp)

	s.cfs = New(fs, fsTmp)
	s.FilesystemSuite.FS = s.cfs
}

func (s *FilesystemSuite) TearDownTest(c *C) {
	_, err := ioutil.ReadDir(s.path)
	c.Assert(err, IsNil)

	err = stdos.RemoveAll(s.path)
	c.Assert(err, IsNil)

	err = stdos.RemoveAll(s.pathTmp)
	c.Assert(err, IsNil)
}
