package tmpfs

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/osfs"
	"gopkg.in/src-d/go-billy.v2/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&FilesystemSuite{})

type FilesystemSuite struct {
	test.FilesystemSuite

	src     billy.Filesystem
	tmp     billy.Filesystem
	srcPath string
	tmpPath string
}

func (s *FilesystemSuite) SetUpTest(c *C) {
	s.srcPath = c.MkDir()
	s.tmpPath = c.MkDir()
	s.src = osfs.New(s.srcPath)
	s.tmp = osfs.New(s.tmpPath)

	s.FilesystemSuite = test.NewFilesystemSuite(New(s.src, s.tmp))
}

func (s *FilesystemSuite) TestTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	filename := f.Name()
	c.Assert(f.Close(), IsNil)

	_, err = os.Stat(filepath.Join(s.srcPath, filename))
	c.Assert(os.IsNotExist(err), Equals, true)

	_, err = os.Stat(filepath.Join(s.tmpPath, filename))
	c.Assert(err, IsNil)
}

func (s *FilesystemSuite) TestNonTempFileInSrcFs(c *C) {
	f, err := s.FS.Create("foo")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	c.Assert(f.Close(), IsNil)

	_, err = os.Stat(filepath.Join(s.srcPath, "foo"))
	c.Assert(err, IsNil)

	_, err = os.Stat(filepath.Join(s.tmpPath, "foo"))
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *FilesystemSuite) TestTempFileCanBeReopened(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("foo"))
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	filename := f.Name()
	c.Assert(f.Close(), IsNil)

	f, err = s.FS.Open(filename)
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filename)

	content, err := ioutil.ReadAll(f)
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "foo")

	c.Assert(f.Close(), IsNil)
}

func (s *FilesystemSuite) TestTempFileCanBeReopenedByOpenFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("foo"))
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	filename := f.Name()
	c.Assert(f.Close(), IsNil)

	f, err = s.FS.OpenFile(filename, os.O_RDONLY, 0)
	c.Assert(err, IsNil)
	c.Assert(f.Name(), Equals, filename)

	content, err := ioutil.ReadAll(f)
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "foo")

	c.Assert(f.Close(), IsNil)
}

func (s *FilesystemSuite) TestStatTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	fi, err := s.FS.Stat(tempFilename)
	c.Assert(err, IsNil)
	c.Assert(fi, NotNil)
}

func (s *FilesystemSuite) TestSymlinkTmpFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	err = s.FS.Symlink(tempFilename, "foo")
	c.Assert(err, NotNil)
}

func (s *FilesystemSuite) TestSymlinkOverTmpFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	err = s.FS.Symlink("foo", tempFilename)
	c.Assert(err, NotNil)
}

func (s *FilesystemSuite) TestRenameFromTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	err = s.FS.Rename(tempFilename, "foo")
	c.Assert(err, IsNil)

	_, err = s.src.Stat("foo")
	c.Assert(err, IsNil)

	_, err = s.tmp.Stat("foo")
	c.Assert(err, NotNil)

	_, err = s.tmp.Stat(tempFilename)
	c.Assert(err, NotNil)
}

func (s *FilesystemSuite) TestCannotRenameToTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	f, err = s.FS.Create("foo")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)
	c.Assert(f.Close(), IsNil)

	err = s.FS.Rename("foo", tempFilename)
	c.Assert(err, NotNil)
}

func (s *FilesystemSuite) TestRemoveTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	err = s.FS.Remove(tempFilename)
	c.Assert(err, IsNil)

	_, err = s.tmp.Stat(tempFilename)
	c.Assert(err, NotNil)
}

func (s *FilesystemSuite) TestCreateTempFile(c *C) {
	f, err := s.FS.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("TEST"))

	tempFilename := f.Name()
	c.Assert(f.Close(), IsNil)

	createdFile, err := s.FS.Create(tempFilename)
	c.Assert(err, IsNil)
	c.Assert(createdFile, NotNil)

	bRead := make([]byte, 4)
	n, err = createdFile.Read(bRead)
	c.Assert(n, Equals, 0)
	c.Assert(err, Equals, io.EOF)
}
