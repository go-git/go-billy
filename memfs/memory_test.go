package memfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/test"
	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MemorySuite struct {
	test.FilesystemSuite
	path string
}

var _ = Suite(&MemorySuite{})

func (s *MemorySuite) SetUpTest(c *C) {
	s.FilesystemSuite = test.NewFilesystemSuite(New())
}

func (s *MemorySuite) TestRootExists(c *C) {
	f, err := s.FS.Stat("/")
	c.Assert(err, IsNil)
	c.Assert(f.IsDir(), Equals, true)
}

func (s *MemorySuite) TestCapabilities(c *C) {
	_, ok := s.FS.(billy.Capable)
	c.Assert(ok, Equals, true)

	caps := billy.Capabilities(s.FS)
	c.Assert(caps, Equals, billy.DefaultCapabilities&^billy.LockCapability)
}

func (s *MemorySuite) TestNegativeOffsets(c *C) {
	f, err := s.FS.Create("negative")
	c.Assert(err, IsNil)

	buf := make([]byte, 100)
	_, err = f.ReadAt(buf, -100)
	c.Assert(err, ErrorMatches, "readat negative: negative offset")

	_, err = f.Seek(-100, io.SeekCurrent)
	c.Assert(err, IsNil)
	_, err = f.Write(buf)
	c.Assert(err, ErrorMatches, "writeat negative: negative offset")
}

func (s *MemorySuite) TestExclusive(c *C) {
	f, err := s.FS.OpenFile("exclusive", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	c.Assert(err, IsNil)

	fmt.Fprint(f, "mememememe")

	err = f.Close()
	c.Assert(err, IsNil)

	_, err = s.FS.OpenFile("exclusive", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	c.Assert(err, ErrorMatches, os.ErrExist.Error())
}

func (s *MemorySuite) TestOrder(c *C) {
	var err error

	files := []string{
		"a",
		"b",
		"c",
	}
	for _, f := range files {
		_, err = s.FS.Create(f)
		c.Assert(err, IsNil)
	}

	attempts := 30
	for n := 0; n < attempts; n++ {
		actual, err := s.FS.ReadDir("")
		c.Assert(err, IsNil)

		for i, f := range files {
			c.Assert(actual[i].Name(), Equals, f)
		}
	}
}

func (s *MemorySuite) TestNotFound(c *C) {
	files, err := s.FS.ReadDir("asdf")
	c.Assert(files, HasLen, 0)
	// JS / wasip have this error message captalised.
	msg := "open /asdf: (N|n)o such file or directory"
	if runtime.GOOS == "windows" {
		msg = `open \\asdf: The system cannot find the file specified\.`
	}
	c.Assert(err, ErrorMatches, msg)
}

func (s *MemorySuite) TestTruncateAppend(c *C) {
	err := util.WriteFile(s.FS, "truncate_append", []byte("file-content"), 0666)
	c.Assert(err, IsNil)

	f, err := s.FS.OpenFile("truncate_append", os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0666)
	c.Assert(err, IsNil)

	n, err := f.Write([]byte("replace"))
	c.Assert(err, IsNil)
	c.Assert(n, Equals, len("replace"))

	err = f.Close()
	c.Assert(err, IsNil)

	data, err := util.ReadFile(s.FS, "truncate_append")
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "replace")
}

func TestReadlink(t *testing.T) {
	tests := []struct {
		name    string
		link    string
		want    string
		wantErr *error
	}{
		{
			name:    "symlink not found",
			link:    "/404",
			wantErr: &os.ErrNotExist,
		},
		{
			name: "self-targeting symlink",
			link: "/self",
			want: "/self",
		},
		{
			name: "symlink",
			link: "/bar",
			want: "/foo",
		},
		{
			name: "symlink to windows path",
			link: "/win",
			want: "c:\\test\\123",
		},
		{
			name: "symlink to network path",
			link: "/net",
			want: "\\test\\123",
		},
	}

	// Cater for memfs not being os-agnostic.
	if runtime.GOOS == "windows" {
		tests[1].want = "\\self"
		tests[2].want = "\\foo"
		tests[3].want = "\\c:\\test\\123"
	}

	fs := New()

	// arrange fs for tests.
	assert.NoError(t, fs.Symlink("/self", "/self"))
	assert.NoError(t, fs.Symlink("/foo", "/bar"))
	assert.NoError(t, fs.Symlink("c:\\test\\123", "/win"))
	assert.NoError(t, fs.Symlink("\\test\\123", "/net"))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fs.Readlink(tc.link)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.ErrorIs(t, err, *tc.wantErr)
			}
		})
	}
}

func TestSymlink(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		link    string
		want    string
		wantErr string
	}{
		{
			name:   "new symlink unexistent target",
			target: "/bar",
			link:   "/foo",
			want:   "/bar",
		},
		{
			name:   "self-targeting symlink",
			target: "/self",
			link:   "/self",
			want:   "/self",
		},
		{
			name:   "new symlink to file",
			target: "/file",
			link:   "/file-link",
			want:   "/file",
		},
		{
			name:   "new symlink to dir",
			target: "/dir",
			link:   "/dir-link",
			want:   "/dir",
		},
		{
			name:   "new symlink to win",
			target: "c:\\foor\\bar",
			link:   "/win",
			want:   "c:\\foor\\bar",
		},
		{
			name:   "new symlink to net",
			target: "\\net\\bar",
			link:   "/net",
			want:   "\\net\\bar",
		},
		{
			name:   "new symlink to net",
			target: "\\net\\bar",
			link:   "/net",
			want:   "\\net\\bar",
		},
		{
			name:    "duplicate symlink",
			target:  "/bar",
			link:    "/foo",
			wantErr: os.ErrExist.Error(),
		},
		{
			name:    "symlink over existing file",
			target:  "/foo/bar",
			link:    "/file",
			want:    "/file",
			wantErr: os.ErrExist.Error(),
		},
	}

	// Cater for memfs not being os-agnostic.
	if runtime.GOOS == "windows" {
		tests[0].want = "\\bar"
		tests[1].want = "\\self"
		tests[2].want = "\\file"
		tests[3].want = "\\dir"
		tests[4].want = "\\c:\\foor\\bar"
	}

	fs := New()

	// arrange fs for tests.
	err := fs.MkdirAll("/dir", 0o600)
	assert.NoError(t, err)
	_, err = fs.Create("/file")
	assert.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := fs.Symlink(tc.target, tc.link)

			if tc.wantErr == "" {
				got, err := fs.Readlink(tc.link)
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestChrootSymlinkResolution(t *testing.T) {
	fs := New()
	require.NoError(t, util.WriteFile(fs, "file", []byte("outer"), 0o644))
	require.NoError(t, util.WriteFile(fs, "dir/file", []byte("outer-dir"), 0o644))

	chroot, err := fs.Chroot("base")
	require.NoError(t, err)
	require.NoError(t, chroot.MkdirAll("nested", 0o755))
	require.NoError(t, chroot.Symlink("../../file", "nested/file-link"))
	require.NoError(t, chroot.Symlink("../../dir", "nested/dir-link"))

	fi, err := chroot.Lstat("nested/file-link")
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0)

	target, err := chroot.Readlink("nested/file-link")
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("../../file"), target)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "create",
			run: func() error {
				f, err := chroot.Create("nested/file-link")
				if err == nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "open file",
			run: func() error {
				f, err := chroot.OpenFile("nested/file-link", os.O_RDONLY, 0)
				if err == nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "open",
			run: func() error {
				f, err := chroot.Open("nested/file-link")
				if err == nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "stat",
			run: func() error {
				_, err := chroot.Stat("nested/file-link")
				return err
			},
		},
		{
			name: "read dir",
			run: func() error {
				_, err := chroot.ReadDir("nested/dir-link")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.run(), billy.ErrCrossedBoundary)
		})
	}

	require.NoError(t, fs.Symlink("file", "root-link"))
	rootChroot, err := fs.Chroot("root-link")
	require.NoError(t, err)

	_, err = rootChroot.Open("/")
	require.ErrorIs(t, err, billy.ErrCrossedBoundary)

	_, err = rootChroot.Stat("/")
	require.ErrorIs(t, err, billy.ErrCrossedBoundary)

	data, err := util.ReadFile(fs, "file")
	require.NoError(t, err)
	assert.Equal(t, []byte("outer"), data)
}

func (s *MemorySuite) TestChrootSymlinkResolutionLoop(c *C) {
	tests := []struct {
		name  string
		setup func(c *C, fs billy.Filesystem)
	}{
		{
			name: "mutual",
			setup: func(c *C, fs billy.Filesystem) {
				c.Assert(fs.Symlink("b", "a"), IsNil)
				c.Assert(fs.Symlink("a", "b"), IsNil)
			},
		},
		{
			name: "self",
			setup: func(c *C, fs billy.Filesystem) {
				c.Assert(fs.Symlink("a", "a"), IsNil)
			},
		},
		{
			name: "self dot relative",
			setup: func(c *C, fs billy.Filesystem) {
				c.Assert(fs.Symlink("./a", "a"), IsNil)
			},
		},
	}

	for _, tt := range tests {
		fs := New()
		tt.setup(c, fs)

		f, err := fs.Open("a")
		c.Assert(f, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "open")

		f, err = fs.Create("a")
		c.Assert(f, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "create")

		fi, err := fs.Stat("a")
		c.Assert(fi, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "stat")
	}
}

func (s *MemorySuite) TestChrootRootSymlinkResolutionLoop(c *C) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "self", target: "root"},
		{name: "self dot relative", target: "./root"},
	}

	for _, tt := range tests {
		fs := New()
		c.Assert(fs.Symlink(tt.target, "root"), IsNil, Commentf("case: %s", tt.name))

		chroot, err := fs.Chroot("root")
		c.Assert(err, IsNil, Commentf("case: %s", tt.name))

		f, err := chroot.Open("/")
		c.Assert(f, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "open")

		fi, err := chroot.Stat("/")
		c.Assert(fi, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "stat")

		entries, err := chroot.ReadDir("/")
		c.Assert(entries, IsNil, Commentf("case: %s", tt.name))
		requireSymlinkLoop(c, err, "readdir")
	}
}

func requireSymlinkLoop(c *C, err error, op string) {
	c.Assert(errors.Is(err, syscall.ELOOP), Equals, true)

	var pathErr *os.PathError
	c.Assert(errors.As(err, &pathErr), Equals, true)
	c.Assert(pathErr.Op, Equals, op)
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		elem []string
		want string
	}{
		{name: "empty", elem: []string{""}, want: ""},
		{name: "c:", elem: []string{"C:"}, want: "C:"},
		{name: "simple rel", elem: []string{"a", "b", "c"}, want: "a/b/c"},
		{name: "simple rel backslash", elem: []string{"\\", "a", "b", "c"}, want: "\\/a/b/c"},
		{name: "simple abs slash", elem: []string{"/", "a", "b", "c"}, want: "/a/b/c"},
		{name: "c: rel", elem: []string{"C:\\", "a", "b", "c"}, want: "C:\\/a/b/c"},
		{name: "c: abs", elem: []string{"/C:\\", "a", "b", "c"}, want: "/C:\\/a/b/c"},
		{name: "\\ rel", elem: []string{"\\\\", "a", "b", "c"}, want: "\\\\/a/b/c"},
		{name: "\\ abs", elem: []string{"/\\\\", "a", "b", "c"}, want: "/\\\\/a/b/c"},
	}

	// Cater for memfs not being os-agnostic.
	if runtime.GOOS == "windows" {
		tests[1].want = "C:."
		tests[2].want = "a\\b\\c"
		tests[3].want = "\\a\\b\\c"
		tests[4].want = "\\a\\b\\c"
		tests[5].want = "C:\\a\\b\\c"
		tests[6].want = "\\C:\\a\\b\\c"
		tests[7].want = "\\\\a\\b\\c"
		tests[8].want = "\\\\\\a\\b\\c"
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := New().Join(tc.elem...)
			assert.Equal(t, tc.want, got)
		})
	}
}

func (s *MemorySuite) TestSymlink(c *C) {
	err := s.FS.Symlink("test", "test")
	c.Assert(err, IsNil)

	f, err := s.FS.Open("test")
	c.Assert(f, IsNil)
	requireSymlinkLoop(c, err, "open")

	fi, err := s.FS.ReadDir("test")
	c.Assert(fi, IsNil)
	requireSymlinkLoop(c, err, "readdir")
}
