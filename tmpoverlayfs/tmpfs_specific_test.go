package tmpfs

import (
	"io"
	"io/ioutil"
	stdos "os"
	"path/filepath"

	"gopkg.in/src-d/go-billy.v2"
	"gopkg.in/src-d/go-billy.v2/osfs"

	. "gopkg.in/check.v1"
)

type SpecificFilesystemSuite struct {
	src     billy.Filesystem
	tmp     billy.Filesystem
	srcPath string
	tmpPath string
}

var _ = Suite(&SpecificFilesystemSuite{})

func (s *SpecificFilesystemSuite) SetUpTest(c *C) {
	s.srcPath = c.MkDir()
	s.tmpPath = c.MkDir()
	s.src = osfs.New(s.srcPath)
	s.tmp = osfs.New(s.tmpPath)
}

func (s *SpecificFilesystemSuite) TestTempFileInTmpFs(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	filename := f.Filename()
	c.Assert(f.Close(), IsNil)

	_, err = stdos.Stat(filepath.Join(s.srcPath, filename))
	c.Assert(stdos.IsNotExist(err), Equals, true)

	_, err = stdos.Stat(filepath.Join(s.tmpPath, filename))
	c.Assert(err, IsNil)
}

func (s *SpecificFilesystemSuite) TestNonTempFileInSrcFs(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.Create("foo")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	c.Assert(f.Close(), IsNil)

	_, err = stdos.Stat(filepath.Join(s.srcPath, "foo"))
	c.Assert(err, IsNil)

	_, err = stdos.Stat(filepath.Join(s.tmpPath, "foo"))
	c.Assert(stdos.IsNotExist(err), Equals, true)
}

func (s *SpecificFilesystemSuite) TestTempFileCanBeReopened(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("foo"))
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	filename := f.Filename()
	c.Assert(f.Close(), IsNil)

	f, err = tmpFs.Open(filename)
	c.Assert(err, IsNil)
	c.Assert(f.Filename(), Equals, filename)

	content, err := ioutil.ReadAll(f)
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "foo")

	c.Assert(f.Close(), IsNil)
}

func (s *SpecificFilesystemSuite) TestTempFileCanBeReopenedByOpenFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("foo"))
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	filename := f.Filename()
	c.Assert(f.Close(), IsNil)

	f, err = tmpFs.OpenFile(filename, stdos.O_RDONLY, 0)
	c.Assert(err, IsNil)
	c.Assert(f.Filename(), Equals, filename)

	content, err := ioutil.ReadAll(f)
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "foo")

	c.Assert(f.Close(), IsNil)
}

func (s *SpecificFilesystemSuite) TestStatTempFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Filename()
	c.Assert(f.Close(), IsNil)

	fi, err := tmpFs.Stat(tempFilename)
	c.Assert(err, IsNil)
	c.Assert(fi, NotNil)
}

func (s *SpecificFilesystemSuite) TestRenameFromTempFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Filename()
	c.Assert(f.Close(), IsNil)

	err = tmpFs.Rename(tempFilename, "foo")
	c.Assert(err, IsNil)

	_, err = s.src.Stat("foo")
	c.Assert(err, IsNil)

	_, err = s.tmp.Stat("foo")
	c.Assert(err, NotNil)

	_, err = s.tmp.Stat(tempFilename)
	c.Assert(err, NotNil)
}

func (s *SpecificFilesystemSuite) TestCannotRenameToTempFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Filename()
	c.Assert(f.Close(), IsNil)

	f, err = tmpFs.Create("foo")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)
	c.Assert(f.Close(), IsNil)

	err = tmpFs.Rename("foo", tempFilename)
	c.Assert(err, NotNil)
}

func (s *SpecificFilesystemSuite) TestRemoveTempFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	tempFilename := f.Filename()
	c.Assert(f.Close(), IsNil)

	err = tmpFs.Remove(tempFilename)
	c.Assert(err, IsNil)

	_, err = s.tmp.Stat(tempFilename)
	c.Assert(err, NotNil)
}

func (s *SpecificFilesystemSuite) TestCreateTempFile(c *C) {
	tmpFs := New(s.src, s.tmp)
	c.Assert(tmpFs, NotNil)

	f, err := tmpFs.TempFile("test-dir", "test-prefix")
	c.Assert(err, IsNil)
	c.Assert(f, NotNil)

	n, err := f.Write([]byte("TEST"))

	tempFilename := f.Filename()
	c.Assert(f.Close(), IsNil)

	createdFile, err := tmpFs.Create(tempFilename)
	c.Assert(err, IsNil)
	c.Assert(createdFile, NotNil)

	bRead := make([]byte, 4)
	n, err = createdFile.Read(bRead)
	c.Assert(n, Equals, 0)
	c.Assert(err, Equals, io.EOF)
}
