package test

import (
	"strings"

	. "gopkg.in/check.v1"
	. "gopkg.in/src-d/go-billy.v2"
)

// TempFileSuite is a convenient test suite to validate any implementation of
// billy.TempFile
type TempFileSuite struct {
	FS interface {
		Basic
		TempFile
	}
}

func (s *TempFileSuite) TestTempFile(c *C) {
	f, err := s.FS.TempFile("", "bar")
	c.Assert(err, IsNil)
	c.Assert(f.Close(), IsNil)

	c.Assert(strings.HasPrefix(f.Filename(), "bar"), Equals, true)
}

func (s *TempFileSuite) TestTempFileWithPath(c *C) {
	f, err := s.FS.TempFile("foo", "bar")
	c.Assert(err, IsNil)
	c.Assert(f.Close(), IsNil)

	c.Assert(strings.HasPrefix(f.Filename(), s.FS.Join("foo", "bar")), Equals, true)
}

func (s *TempFileSuite) TestTempFileFullWithPath(c *C) {
	f, err := s.FS.TempFile("/foo", "bar")
	c.Assert(err, IsNil)
	c.Assert(f.Close(), IsNil)

	c.Assert(strings.HasPrefix(f.Filename(), s.FS.Join("foo", "bar")), Equals, true)
}

func (s *TempFileSuite) TestRemoveTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)

	fn := f.Filename()
	c.Assert(err, IsNil)
	c.Assert(f.Close(), IsNil)

	c.Assert(s.FS.Remove(fn), IsNil)
}
