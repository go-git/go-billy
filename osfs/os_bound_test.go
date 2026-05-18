//go:build !wasm

/*
   Copyright 2022 The Flux authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package osfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundOSCapabilities(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

	assert.Equal(t, boundCapabilities(), fs.Capabilities())
}

func TestFromRoot(t *testing.T) {
	t.Parallel()

	t.Run("valid root", func(t *testing.T) {
		t.Parallel()
		root, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, root.Close()) })

		fs, err := FromRoot(root)
		require.NoError(t, err)
		assert.IsType(t, &RootOS{}, fs)
		assert.Equal(t, root.Name(), fs.Root())

		f, err := fs.Create("test-file")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = fs.Stat("test-file")
		require.NoError(t, err)
	})

	t.Run("nil root", func(t *testing.T) {
		t.Parallel()
		_, err := FromRoot(nil)
		require.Error(t, err)
	})

	t.Run("closed root", func(t *testing.T) {
		t.Parallel()
		root, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, root.Close())

		fs, err := FromRoot(root)
		require.NoError(t, err)
		assert.Equal(t, root.Name(), fs.Root())

		_, err = fs.Stat(".")
		require.ErrorContains(t, err, "file already closed")
	})
}

// TestOpenAbsSymlinkInsideRoot verifies that Open can follow a symlink whose
// target is an absolute path pointing inside the root. os.Root rejects such
// symlinks because the absolute target appears to escape the root. BoundOS
// detects this, resolves the link to a relative path, and retries.
func TestOpenAbsSymlinkInsideRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	absTarget := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(absTarget, []byte("content"), 0o600))
	require.NoError(t, os.Symlink(absTarget, filepath.Join(dir, "link")))

	// Prove os.Root alone rejects the absolute symlink target.
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { root.Close() })

	_, err = root.Open("link")
	require.Error(t, err, "os.Root.Open must reject an absolute symlink target inside the root")
	require.ErrorContains(t, err, ErrPathEscapesParent.Error())

	// BoundOS resolves the absolute target to a relative path and succeeds.
	bfs := newBoundOS(dir)
	f, err := bfs.Open("link")
	require.NoError(t, err)

	got := make([]byte, 7)
	_, err = f.Read(got)
	require.NoError(t, err)
	assert.Equal(t, "content", string(got))
	require.NoError(t, f.Close())
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(t *testing.T, dir string) billy.Filesystem
		wantErr  error
	}{
		{
			name: "file: rel same dir",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600))
				return newBoundOS(dir)
			},
			filename: "test-file",
		},
		{
			name: "file: absolute fs path",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600))
				return newBoundOS(dir)
			},
			filename: "/abs-test-file",
		},
		{
			name: "file: host absolute path inside root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600))
				return newBoundOS(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "file: parent traversal clamps to root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600))
				return newBoundOS(dir)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "symlink: absolute target inside root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				target := filepath.Join(dir, "target-file")
				require.NoError(t, os.WriteFile(target, []byte("anything"), 0o600))
				require.NoError(t, os.Symlink(target, filepath.Join(dir, "symlink")))
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "symlink: relative target inside root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "target-file"), []byte("anything"), 0o600))
				require.NoError(t, os.Symlink("target-file", filepath.Join(dir, "symlink")))
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "symlink: absolute target outside root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink")))
				return newBoundOS(dir)
			},
			filename: "symlink",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "symlink: relative target outside root",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.Symlink("../../../../../../outside/cwd", filepath.Join(dir, "symlink")))
				return newBoundOS(dir)
			},
			filename: "symlink",
			wantErr:  ErrPathEscapesParent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var fs billy.Filesystem = newBoundOS(dir)
			if tt.before != nil {
				fs = tt.before(t, dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			f, err := fs.Open(filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, f)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, f)
			require.NoError(t, f.Close())
		})
	}
}

func TestOpenPreservesBackslashFilenamesOnNonWindows(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("backslash is a path separator on windows")
	}

	dir := t.TempDir()
	fs := newBoundOS(dir)

	tests := []struct {
		filename string
		openPath string
	}{
		{filename: `.\test-file`, openPath: `.\test-file`},
		{filename: `\test-file`, openPath: `./\test-file`},
	}
	for _, tt := range tests {
		t.Run(tt.openPath, func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, tt.filename), []byte("anything"), 0o600))

			f, err := fs.Open(tt.openPath)
			require.NoError(t, err)
			require.NotNil(t, f)
			require.NoError(t, f.Close())
		})
	}
}

func TestSymlink(t *testing.T) {
	defer umask(0)()

	tests := []struct {
		name    string
		link    string
		target  string
		makeAbs bool
	}{
		{name: "link to abs valid target", link: "symlink", target: filepath.FromSlash("/etc/passwd")},
		{name: "abs link to abs valid target", link: "symlink", target: filepath.FromSlash("/etc/passwd"), makeAbs: true},
		{name: "dot link to abs valid target", link: "./symlink", target: filepath.FromSlash("/etc/passwd")},
		{name: "auto create dir", link: "new-dir/symlink", target: filepath.FromSlash("../../../some/random/path")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir)

			link := tt.link
			if tt.makeAbs {
				link = filepath.Join(dir, tt.link)
			}

			require.NoError(t, fs.Symlink(tt.target, link))

			got, err := os.Readlink(filepath.Join(dir, tt.link))
			require.NoError(t, err)
			assert.Equal(t, tt.target, got)
		})
	}
}

func TestTempFile(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

	f, err := fs.TempFile("", "prefix")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(f.Name(), filepath.Join(".tmp", "prefix")), f.Name())
	require.NoError(t, f.Close())

	f, err = fs.TempFile("/above/cwd", "prefix")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(f.Name(), filepath.Join("above", "cwd", "prefix")), f.Name())
	require.NoError(t, f.Close())

	f, err = fs.TempFile("../../../above/cwd", "prefix")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(f.Name(), filepath.Join("above", "cwd", "prefix")), f.Name())
	require.NoError(t, f.Close())
}

func TestDefaultTempFileNameCanBeReopened(t *testing.T) {
	f, err := Default.TempFile("", "go-billy-")
	require.NoError(t, err)

	name := f.Name()
	require.True(t, filepath.IsAbs(hostPath(name)), name)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = Default.Remove(name) })

	reopened, err := Default.Open(name)
	require.NoError(t, err)
	require.NoError(t, reopened.Close())
}

func TestChroot(t *testing.T) {
	tmp := t.TempDir()
	fs := newBoundOS(tmp)

	chrooted, err := fs.Chroot("test")
	require.NoError(t, err)
	assert.IsType(t, &BoundOS{}, chrooted)
	assert.Equal(t, filepath.Join(tmp, "test"), chrooted.Root())
}

func TestChrootMissingPathDoesNotCreate(t *testing.T) {
	tmp := t.TempDir()
	fs := newBoundOS(tmp)

	chrooted, err := fs.Chroot("missing")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "missing"), chrooted.Root())

	_, err = os.Stat(filepath.Join(tmp, "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestChrootDotKeepsCleanRoot(t *testing.T) {
	tmp := t.TempDir()
	fs := newBoundOS(tmp)

	chrooted, err := fs.Chroot(".")
	require.NoError(t, err)
	assert.Equal(t, tmp, chrooted.Root())
}

func TestChrootClampsParentTraversalToChildRoot(t *testing.T) {
	tmp := t.TempDir()
	fs := newBoundOS(tmp)
	require.NoError(t, util.WriteFile(fs, "bar", []byte("root"), 0o644))
	require.NoError(t, util.WriteFile(fs, "foo/bar", []byte("chroot"), 0o644))

	chrooted, err := fs.Chroot("foo")
	require.NoError(t, err)

	got, err := util.ReadFile(chrooted, "../bar")
	require.NoError(t, err)
	assert.Equal(t, []byte("chroot"), got)
}

func TestRoot(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

	assert.Equal(t, dir, fs.Root())
}

func TestStatBaseFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(name, []byte("content"), 0o600))

	fs := newBoundOS(name)
	for _, p := range []string{"", ".", "./", "./.", "/"} {
		t.Run(fmt.Sprintf("path=%q", p), func(t *testing.T) {
			fi, err := fs.Stat(p)
			require.NoError(t, err)
			assert.False(t, fi.IsDir())
		})
	}
}

func TestEmptyBaseUsesOSRoot(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(name, []byte("content"), 0o600))

	fs := newBoundOS("")
	assert.Equal(t, string(os.PathSeparator), fs.Root())

	f, err := fs.Open(name)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestMkdirAllCreatesMissingBaseDir(t *testing.T) {
	base := filepath.Join(t.TempDir(), "repo.git")
	fs := newBoundOS(base)

	require.NoError(t, fs.MkdirAll(filepath.Join("objects", "info"), 0o700))
	mustExist(t, filepath.Join(base, "objects", "info"))
}

func TestCreateCreatesMissingBaseDir(t *testing.T) {
	base := filepath.Join(t.TempDir(), "repo.git")
	fs := newBoundOS(base)

	f, err := fs.Create("config")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	mustExist(t, filepath.Join(base, "config"))
}

func TestCreateCreatesMissingBaseDirAncestors(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "missing", "parents", "repo.git")
	fs := newBoundOS(base)

	_, err := os.Stat(filepath.Join(root, "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)

	f, err := fs.Create(filepath.Join("objects", "info", "alternates"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mustExist(t, filepath.Join(base, "objects", "info", "alternates"))
}

func TestReadlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	target := filepath.FromSlash("/etc/passwd")
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "symlink")))

	got, err := fs.Readlink("symlink")
	require.NoError(t, err)
	assert.Equal(t, filepath.ToSlash(target), got)

	_, err = fs.Readlink("../../symlink")
	require.NoError(t, err)
}

func TestLstat(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	require.NoError(t, os.Symlink("target-file", filepath.Join(dir, "link")))

	fi, err := fs.Lstat("link")
	require.NoError(t, err)
	assert.Equal(t, "link", fi.Name())
	assert.NotZero(t, fi.Mode()&os.ModeSymlink)
}

func TestStatPreventsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	require.NoError(t, os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink")))

	fi, err := fs.Stat("symlink")
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(t, fi)
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		before   func(t *testing.T, dir string) billy.Filesystem
		wantErr  error
	}{
		{
			name:     "inexistent dir",
			filename: "inexistent",
			wantErr:  os.ErrNotExist,
		},
		{
			name:     "base dir as dot",
			filename: ".",
			wantErr:  billy.ErrBaseDirCannotBeRemoved,
		},
		{
			name: "same dir file",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600))
				return newBoundOS(dir)
			},
			filename: "test-file",
		},
		{
			name: "symlink is removed without touching target",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "target-file"), []byte("target"), 0o600))
				require.NoError(t, os.Symlink("target-file", filepath.Join(dir, "link")))
				return newBoundOS(dir)
			},
			filename: "link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var fs billy.Filesystem = newBoundOS(dir)
			if tt.before != nil {
				fs = tt.before(t, dir)
			}

			err := fs.Remove(tt.filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRemoveAll(t *testing.T) {
	dir := t.TempDir()
	fs := newTestBoundOS(t, dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "parent", "child"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "parent", "child", "file"), []byte("anything"), 0o600))

	require.NoError(t, fs.RemoveAll("parent"))
	_, err := os.Stat(filepath.Join(dir, "parent"))
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestRemoveAllRemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newTestBoundOS(t, dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "target-dir"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "target-dir", "file"), []byte("target"), 0o600))
	require.NoError(t, os.Symlink("target-dir", filepath.Join(dir, "link")))

	require.NoError(t, fs.RemoveAll("link"))
	_, err := os.Lstat(filepath.Join(dir, "link"))
	require.ErrorIs(t, err, os.ErrNotExist)
	requireFileContents(t, filepath.Join(dir, "target-dir", "file"), []byte("target"))
}

func TestRemoveBaseDir(t *testing.T) {
	tests := []string{"", ".", "./", "/..", "/foo/.."}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			fs := newTestBoundOS(t, dir)
			require.ErrorIs(t, fs.Remove(path), billy.ErrBaseDirCannotBeRemoved)
			require.ErrorIs(t, fs.RemoveAll(path), billy.ErrBaseDirCannotBeRemoved)
			mustExist(t, dir)
		})
	}
}

func TestRemoveBaseDirAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	fs := newTestBoundOS(t, dir)

	require.ErrorIs(t, fs.Remove(dir), billy.ErrBaseDirCannotBeRemoved)
	require.ErrorIs(t, fs.RemoveAll(dir), billy.ErrBaseDirCannotBeRemoved)
	mustExist(t, dir)
}

func TestJoin(t *testing.T) {
	fs := newBoundOS(t.TempDir())

	assert.Equal(t, filepath.FromSlash("/a/b/c"), fs.Join("/a", "b/c"))
	assert.Equal(t, "", fs.Join())
}

func TestReadDir(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1"), []byte{}, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file2"), []byte{}, 0o600))

	dirs, err := fs.ReadDir(".")
	require.NoError(t, err)
	assert.Len(t, dirs, 2)

	dirs, err = fs.ReadDir("/")
	require.NoError(t, err)
	assert.Len(t, dirs, 2)
}

func TestReadDirPreventsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	require.NoError(t, os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink")))

	dirs, err := fs.ReadDir("symlink")
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(t, dirs)
}

func TestMkdirAll(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	require.NoError(t, os.MkdirAll(cwd, 0o700))
	fs := newBoundOS(cwd)

	require.NoError(t, fs.MkdirAll("abc", 0o700))
	mustExist(t, filepath.Join(cwd, "abc"))
}

func TestMkdirAllPreventsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on windows")
	}

	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(cwd, 0o700))
	require.NoError(t, os.Mkdir(outside, 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(cwd, "symlink")))

	fs := newBoundOS(cwd)
	err := fs.MkdirAll(filepath.Join(cwd, "symlink", "new-dir"), 0o700)
	require.ErrorIs(t, err, ErrPathEscapesParent)
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	oldFile := "old-file"
	newFile := filepath.Join("newdir", "newfile")

	f, err := fs.Create(oldFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, fs.Rename(oldFile, newFile))
	mustExist(t, filepath.Join(dir, newFile))
}

func TestRenameAbsoluteSource(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

	f, err := fs.TempFile("", "tmp-")
	require.NoError(t, err)
	_, err = f.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	dst := filepath.Join("objects", "a8", "finalfile")
	require.NoError(t, fs.Rename(f.Name(), dst))

	got, err := os.ReadFile(filepath.Join(dir, dst))
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))
}

func TestRenameRenamesSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	target := filepath.Join(dir, "target-file")
	link := filepath.Join(dir, "link")
	renamed := filepath.Join(dir, "renamed-link")

	require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
	require.NoError(t, os.Symlink("target-file", link))

	require.NoError(t, fs.Rename("link", "renamed-link"))
	_, err := os.Lstat(link)
	require.ErrorIs(t, err, os.ErrNotExist)

	fi, err := os.Lstat(renamed)
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSymlink)
	got, err := os.Readlink(renamed)
	require.NoError(t, err)
	assert.Equal(t, "target-file", got)
	requireFileContents(t, target, []byte("target"))
}

func TestRenameBaseDir(t *testing.T) {
	for _, from := range []string{"", ".", "./", "/..", "/foo/.."} {
		t.Run(from, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir)

			err := fs.Rename(from, "renamed")
			require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRenamed)
			mustExist(t, dir)
		})
	}
}

func TestRenamePreventsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on windows")
	}

	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(cwd, 0o700))
	require.NoError(t, os.Mkdir(outside, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "old-file"), []byte("data"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(cwd, "symlink")))

	fs := newBoundOS(cwd)
	err := fs.Rename("old-file", filepath.Join("symlink", "new-file"))
	require.ErrorIs(t, err, ErrPathEscapesParent)
}

func mustExist(t *testing.T, filename string) {
	t.Helper()
	fi, err := os.Stat(filename)
	require.NoError(t, err)
	require.NotNil(t, fi)
}

func requireFileContents(t *testing.T, filename string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func newTestBoundOS(t *testing.T, dir string, opts ...Option) *BoundOS {
	t.Helper()

	if len(opts) == 0 {
		return newBoundOS(dir)
	}
	fs, ok := New(dir, opts...).(*BoundOS)
	require.True(t, ok)
	return fs
}
