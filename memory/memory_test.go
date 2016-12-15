package memory

import (
	"testing"

	. "gopkg.in/check.v1"
	"srcd.works/go-billy.v1/test"
)

func Test(t *testing.T) { TestingT(t) }

type MemorySuite struct {
	test.FilesystemSuite
	path string
}

var _ = Suite(&MemorySuite{})

func (s *MemorySuite) SetUpTest(c *C) {
	s.FilesystemSuite.Fs = New()
}

func (s *MemorySuite) TestTempFileMaxTempFiles(c *C) {
	for i := 0; i < maxTempFiles; i++ {
		f, err := s.FilesystemSuite.Fs.TempFile("", "")
		c.Assert(err, IsNil)
		c.Assert(f, NotNil)
	}

	f, err := s.FilesystemSuite.Fs.TempFile("", "")
	c.Assert(err, NotNil)
	c.Assert(f, IsNil)
}
