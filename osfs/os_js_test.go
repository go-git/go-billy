// +build js

package osfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type OSSuite struct {
	test.FilesystemSuite
	path        string
	tempCounter int
}

var _ = Suite(&OSSuite{})

func (s *OSSuite) SetUpTest(c *C) {
	s.tempCounter++
	s.path = fmt.Sprintf("test_%d", s.tempCounter)
	s.FilesystemSuite = test.NewFilesystemSuite(New(s.path))
}

func (s *OSSuite) TestOpenDoesNotCreateDir(c *C) {
	_, err := s.FS.Open("dir/non-existent")
	c.Assert(err, NotNil)

	_, err = s.FS.Stat(filepath.Join(s.path, "dir"))
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *OSSuite) TestCapabilities(c *C) {
	_, ok := s.FS.(billy.Capable)
	c.Assert(ok, Equals, true)

	caps := billy.Capabilities(s.FS)
	c.Assert(caps, Equals, billy.DefaultCapabilities&^billy.LockCapability)
}
