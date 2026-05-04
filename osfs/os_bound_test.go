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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundOSCapabilities(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir, true)
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := billy.Capabilities(fs)
	assert.Equal(t, billy.AllCapabilities, caps)
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "file: rel same dir",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "file: dot rel same dir",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "./test-file",
		},
		{
			name: "file: dot rel same dir without deduplication",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, false)
			},
			filename: "./test-file",
		},
		{
			name: "file: rel path to above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "file: rel path to below cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Mkdir(filepath.Join(dir, "sub"), 0o700)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "sub/rel-below-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "sub/rel-below-cwd",
		},
		{
			name: "file: abs inside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "file: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir, true)
			},
			filename: "/some/path/outside/cwd",
			wantErr:  notFoundError(),
		},
		{
			name: "symlink: same dir",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(target, filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "symlink: rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../outside/cwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
		{
			name: "symlink: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			fi, err := fs.Open(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(fi)
			} else {
				require.NoError(t, err)
				assert.NotNil(fi)
				require.NoError(t, fi.Close())
			}
		})
	}
}

func TestOpenPreservesBackslashFilenamesOnNonWindows(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("backslash is a path separator on windows")
	}

	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	tests := []struct {
		filename string
		openPath string
	}{
		{filename: `.\test-file`, openPath: `.\test-file`},
		{filename: `\test-file`, openPath: `./\test-file`},
	}
	for _, tt := range tests {
		t.Run(tt.openPath, func(t *testing.T) {
			err := os.WriteFile(filepath.Join(dir, tt.filename), []byte("anything"), 0o600)
			require.NoError(t, err)

			fi, err := fs.Open(tt.openPath)
			require.NoError(t, err)
			require.NotNil(t, fi)
			require.NoError(t, fi.Close())
		})
	}
}

func TestWindowsOpenBackslashDotPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("backslash is only a path separator on windows")
	}

	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name     string
		openPath string
		wantName string
	}{
		{name: "dot backslash file", openPath: `.\test-file`, wantName: "test-file"},
		{name: "dot backslash parent", openPath: `.\..`, wantName: "."},
		{name: "absolute parent", openPath: `\..`, wantName: "."},
		{name: "absolute child parent", openPath: `\foo\..`, wantName: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := fs.Open(tt.openPath)
			require.NoError(t, err)
			require.NotNil(t, f)
			assert.Equal(t, filepath.Clean(tt.wantName), filepath.Clean(f.Name()))
			require.NoError(t, f.Close())
		})
	}
}

func Test_Symlink(t *testing.T) {
	// The umask value set at OS level can impact this test, so
	// it is set to 0 during the duration of this test and then
	// reverted back to the original value.
	// Outside of linux this is a no-op.
	defer umask(0)()

	tests := []struct {
		name        string
		link        string
		target      string
		before      func(dir string) billy.Filesystem
		wantStatErr string
	}{
		{
			name:   "link to abs valid target",
			link:   "symlink",
			target: filepath.FromSlash("/etc/passwd"),
		},
		{
			name:   "dot link to abs valid target",
			link:   "./symlink",
			target: filepath.FromSlash("/etc/passwd"),
		},
		{
			name:   "link to abs inexistent target",
			link:   "symlink",
			target: filepath.FromSlash("/some/random/path"),
		},
		{
			name:   "link to rel valid target",
			link:   "symlink",
			target: filepath.FromSlash("../../../../../../../../../etc/passwd"),
		},
		{
			name:   "link to rel inexistent target",
			link:   "symlink",
			target: filepath.FromSlash("../../../some/random/path"),
		},
		{
			name:   "auto create dir",
			link:   "new-dir/symlink",
			target: filepath.FromSlash("../../../some/random/path"),
		},
		{
			name: "keep dir filemode if exists",
			link: "new-dir/symlink",
			before: func(dir string) billy.Filesystem {
				err := os.Mkdir(filepath.Join(dir, "new-dir"), 0o701)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			target: filepath.FromSlash("../../../some/random/path"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			// Even if CWD is changed outside of the fs instance,
			// the base dir must still be observed.
			err := os.Chdir(os.TempDir())
			require.NoError(t, err)

			link := filepath.Join(dir, tt.link)

			diBefore, _ := os.Lstat(filepath.Dir(link))

			err = fs.Symlink(tt.target, tt.link)
			require.NoError(t, err)

			fi, err := os.Lstat(link)
			if tt.wantStatErr != "" {
				require.ErrorContains(t, err, tt.wantStatErr)
			} else {
				require.NoError(t, err)
				assert.NotNil(fi)
			}

			got, err := os.Readlink(link)
			require.NoError(t, err)
			assert.Equal(tt.target, got)

			diAfter, err := os.Lstat(filepath.Dir(link))
			require.NoError(t, err)

			if diBefore != nil {
				assert.Equal(diBefore.Mode(), diAfter.Mode())
			}
		})
	}
}

func TestTempFile(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	f, err := fs.TempFile("", "prefix")
	require.NoError(t, err)
	assert.NotNil(f)
	assert.Contains(f.Name(), os.TempDir())
	require.NoError(t, f.Close())

	f, err = fs.TempFile("/above/cwd", "prefix")
	require.NoError(t, err)
	assert.NotNil(f)
	assert.Contains(f.Name(), filepath.Join("above", "cwd", "prefix"))
	require.NoError(t, f.Close())

	dir = os.TempDir()
	// For windows, volume name must be removed.
	if v := filepath.VolumeName(dir); v != "" {
		dir = strings.TrimPrefix(dir, v)
	}

	f, err = fs.TempFile(dir, "prefix")
	require.NoError(t, err)
	assert.NotNil(f)
	assert.Contains(f.Name(), filepath.Join(strings.TrimLeft(dir, `/\`), "prefix"))
	require.NoError(t, f.Close())
}

func TestChroot(t *testing.T) {
	assert := assert.New(t)
	tmp := t.TempDir()
	fs := newBoundOS(tmp, true)

	f, err := fs.Chroot("test")
	require.NoError(t, err)
	assert.NotNil(f)
	assert.Equal(filepath.Join(tmp, "test"), f.Root())
	assert.IsType(&BoundOS{}, f)
}

func TestRoot(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	root := fs.Root()
	assert.Equal(dir, root)
}

func TestReadLink(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		makeAbs         bool
		expected        string
		makeExpectedAbs bool
		before          func(dir string) billy.Filesystem
		wantErr         string
	}{
		{
			name: "symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			expected: filepath.FromSlash("/etc/passwd"),
		},
		{
			name:     "file: rel parent traversal is clamped to cwd",
			filename: "../../file",
			wantErr:  notFoundError(),
		},
		{
			name: "symlink: abs symlink pointing outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			expected: filepath.FromSlash("/etc/passwd"),
		},
		{
			name: "symlink: dir pointing outside cwd",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outside := filepath.Join(dir, "outside-cwd")

				err := os.Mkdir(cwd, 0o700)
				require.NoError(t, err)
				err = os.Mkdir(outside, 0o700)
				require.NoError(t, err)

				err = os.Symlink(outside, filepath.Join(cwd, "symlink"))
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(outside, "file"), []byte("anything"), 0o600)
				require.NoError(t, err)

				return newBoundOS(cwd, true)
			},
			filename: "current-dir/symlink/file",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
		{
			name: "symlink: within cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				cwdAlt := filepath.Join(dir, "symlink-altdir")
				cwdTarget := filepath.Join(dir, "cwd-target")

				err := os.MkdirAll(cwdTarget, 0o700)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwd)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwdAlt)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				require.NoError(t, err)
				return newBoundOS(cwd, true)
			},
			filename:        "symlink-file",
			expected:        filepath.FromSlash("cwd-target/file"),
			makeExpectedAbs: true,
		},
		{
			name: "symlink: outside cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem { //nolint
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				err := os.Mkdir(cwdTarget, 0o700)
				require.NoError(t, err)
				err = os.Mkdir(outsideDir, 0o700)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwd)
				require.NoError(t, err)
				err = os.Symlink(outsideDir, outside)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
				require.NoError(t, err)
				return newBoundOS(cwd, true)
			},
			filename: "symlink-outside/symlink-file",
			wantErr:  notFoundError(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			expected := tt.expected
			if tt.makeExpectedAbs {
				expected = filepath.Join(dir, expected)
			}

			got, err := fs.Readlink(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(got)
			} else {
				require.NoError(t, err)
				assert.Equal(expected, got)
			}
		})
	}
}

func TestReadlinkReadsSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir, true)
	target := filepath.Join(dir, "target-file")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("target"), 0o600)
	require.NoError(t, err)
	err = os.Symlink("target-file", link)
	require.NoError(t, err)

	got, err := fs.Readlink("link")
	require.NoError(t, err)
	assert.Equal(t, "target-file", got)
	requireFileContents(t, target, []byte("target"))
}

func TestLstat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  false,
		},
		{
			name: "symlink: within cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				cwdAlt := filepath.Join(dir, "symlink-altdir")
				cwdTarget := filepath.Join(dir, "cwd-target")

				err := os.MkdirAll(cwdTarget, 0o700)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwd)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwdAlt)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				require.NoError(t, err)
				return newBoundOS(cwd, true)
			},
			filename: "symlink-file",
			makeAbs:  false,
		},
		{
			name: "symlink: outside cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem { //nolint
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				err := os.Mkdir(cwdTarget, 0o700)
				require.NoError(t, err)
				err = os.Mkdir(outsideDir, 0o700)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				require.NoError(t, err)
				err = os.Symlink(cwdTarget, cwd)
				require.NoError(t, err)
				err = os.Symlink(outsideDir, outside)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
				require.NoError(t, err)

				return newBoundOS(cwd, true)
			},
			filename: "symlink-outside/symlink-file",
			makeAbs:  false,
			wantErr:  notFoundError(),
		},
		{
			name:     "path: rel parent traversal is clamped to cwd",
			filename: "../../file",
			wantErr:  notFoundError(),
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  notFoundError(),
		},
		{
			name: "file: rel",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "file: dot rel",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "./test-file",
		},
		{
			name: "file: abs",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}
			fi, err := fs.Lstat(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(fi)
			} else {
				require.NoError(t, err)
				assert.NotNil(fi)
				assert.Equal(filepath.Base(tt.filename), fi.Name())
			}
		})
	}
}

func TestLstatStatsSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir, true)
	target := filepath.Join(dir, "target-file")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("target"), 0o600)
	require.NoError(t, err)
	err = os.Symlink("target-file", link)
	require.NoError(t, err)

	fi, err := fs.Lstat("link")
	require.NoError(t, err)
	assert.Equal(t, "link", fi.Name())
	assert.NotZero(t, fi.Mode()&os.ModeSymlink)
	requireFileContents(t, target, []byte("target"))
}

func TestNoFollowOperationsAllowMissingPathUnderSymlinkedBase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	base := filepath.Join(dir, "base")
	err := os.Mkdir(target, 0o700)
	require.NoError(t, err)
	err = os.Symlink(target, base)
	require.NoError(t, err)

	fs := newBoundOS(base, true)

	_, err = fs.Lstat("/etc/passwd")
	require.ErrorContains(t, err, notFoundError())
	assert.NotContains(t, err.Error(), "path outside base dir")

	err = fs.Remove("/some/path/outside/cwd")
	require.ErrorContains(t, err, notFoundError())
	assert.NotContains(t, err.Error(), "path outside base dir")

	err = os.WriteFile(filepath.Join(target, "old-file"), []byte("renamed"), 0o600)
	require.NoError(t, err)
	err = fs.Rename("old-file", filepath.Join("newdir", "newfile"))
	require.NoError(t, err)
	requireFileContents(t, filepath.Join(target, "newdir", "newfile"), []byte("renamed"))
	mustNotExist(filepath.Join(target, "old-file"))
}

func TestNoFollowOperationsContainSymlinkedParentOutsideBase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on windows")
	}

	dir := t.TempDir()
	base := filepath.Join(dir, "base")
	outside := filepath.Join(dir, "outside")
	err := os.Mkdir(base, 0o700)
	require.NoError(t, err)
	err = os.Mkdir(outside, 0o700)
	require.NoError(t, err)
	err = os.Symlink(outside, filepath.Join(base, "link"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(base, "file"), []byte("source"), 0o600)
	require.NoError(t, err)

	fs := newBoundOS(base, true)

	_, err = fs.Lstat(filepath.Join("link", "missing", "file"))
	require.ErrorContains(t, err, notFoundError())
	assert.NotContains(t, err.Error(), "path outside base dir")

	err = fs.Remove(filepath.Join("link", "missing", "file"))
	require.ErrorContains(t, err, notFoundError())
	assert.NotContains(t, err.Error(), "path outside base dir")

	err = fs.Rename("file", filepath.Join("link", "missing", "file"))
	require.NoError(t, err)

	parent, err := securejoin.SecureJoin(base, filepath.Join("link", "missing"))
	require.NoError(t, err)
	requireFileContents(t, filepath.Join(parent, "file"), []byte("source"))
	mustNotExist(filepath.Join(base, "file"))
	mustNotExist(filepath.Join(outside, "missing", "file"))
}

func TestStat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			wantErr:  notFoundError(),
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			wantErr:  notFoundError(),
		},

		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  false,
			wantErr:  notFoundError(),
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  notFoundError(),
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  notFoundError(),
		},
		{
			name: "rel file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "rel dot dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir, true)
			},
			filename: ".",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			fi, err := fs.Stat(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(fi)
			} else {
				require.NoError(t, err)
				assert.NotNil(fi)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name:     "path: rel pointing outside cwd w forward slash",
			filename: "/some/path/outside/cwd",
			wantErr:  notFoundError(),
		},
		{
			name:     "path: rel parent traversal is clamped to cwd",
			filename: "../../../../path/outside/cwd",
			wantErr:  notFoundError(),
		},
		{
			name: "inexistent dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir, true)
			},
			filename: "inexistent",
			wantErr:  notFoundError(),
		},
		{
			name:     "base dir as empty path",
			filename: "",
			wantErr:  "base dir cannot be removed",
		},
		{
			name: "base dir as dot",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir, true)
			},
			filename: ".",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as dot slash",
			filename: "./",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as absolute parent",
			filename: "/..",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as absolute child parent",
			filename: "/foo/..",
			wantErr:  "base dir cannot be removed",
		},
		{
			name: "same dir file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "symlink: same dir",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(target, filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel parent traversal removes file under cwd",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing outside removes link",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outsideFile := filepath.Join(dir, "outside-cwd/file")

				err := os.Mkdir(cwd, 0o700)
				require.NoError(t, err)
				err = os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				require.NoError(t, err)
				err = os.WriteFile(outsideFile, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(outsideFile, filepath.Join(cwd, "remove-abs-symlink"))
				require.NoError(t, err)
				return newBoundOS(cwd, true)
			},
			filename: "remove-abs-symlink",
		},
		{
			name: "rel symlink: pointing outside is forced to descend",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outsideFile := filepath.Join(dir, "outside-cwd", "file2")

				err := os.Mkdir(cwd, 0o700)
				require.NoError(t, err)
				err = os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				require.NoError(t, err)
				err = os.WriteFile(outsideFile, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join("..", "outside-cwd", "file2"), filepath.Join(cwd, "remove-abs-symlink2"))
				require.NoError(t, err)
				return newBoundOS(cwd, true)
			},
			filename: "remove-rel-symlink",
			wantErr:  notFoundError(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.Remove(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRemoveRemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir, true)
	target := filepath.Join(dir, "target-file")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("target"), 0o600)
	require.NoError(t, err)
	err = os.Symlink("target-file", link)
	require.NoError(t, err)

	err = fs.Remove("link")
	require.NoError(t, err)
	_, err = os.Lstat(link)
	require.ErrorIs(t, err, os.ErrNotExist)
	requireFileContents(t, target, []byte("target"))
}

func TestWindowsRemoveBackslashDotPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("backslash is only a path separator on windows")
	}

	t.Run("dot backslash file", func(t *testing.T) {
		dir := t.TempDir()
		fs := newBoundOS(dir, true)

		filename := filepath.Join(dir, "test-file")
		err := os.WriteFile(filename, []byte("anything"), 0o600)
		require.NoError(t, err)

		err = fs.Remove(`.\test-file`)
		require.NoError(t, err)
		mustNotExist(filename)
	})

	for _, path := range windowsBackslashBaseDirAliases() {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			err := fs.Remove(path)
			require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRemoved)
			mustExist(dir)
		})
	}
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(t *testing.T, dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "parent with children",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.MkdirAll(filepath.Join(dir, "parent", "children"), 0o700)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "parent",
		},
		{
			name:     "inexistent dir",
			filename: "inexistent",
		},
		{
			name:     "base dir as empty path",
			filename: "",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as dot",
			filename: ".",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as dot slash",
			filename: "./",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as absolute parent",
			filename: "/..",
			wantErr:  "base dir cannot be removed",
		},
		{
			name:     "base dir as absolute child parent",
			filename: "/foo/..",
			wantErr:  "base dir cannot be removed",
		},
		{
			name: "same dir file",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "same dir symlink",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(target, filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel parent traversal removes file under cwd",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs, ok := newBoundOS(dir, true).(*BoundOS)
			assert.True(ok)

			if tt.before != nil {
				fs, ok = tt.before(t, dir).(*BoundOS)
				assert.True(ok)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.RemoveAll(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRemoveAllRemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	fs, ok := newBoundOS(dir, true).(*BoundOS)
	require.True(t, ok)
	target := filepath.Join(dir, "target-dir")
	link := filepath.Join(dir, "link")

	err := os.Mkdir(target, 0o700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(target, "file"), []byte("target"), 0o600)
	require.NoError(t, err)
	err = os.Symlink("target-dir", link)
	require.NoError(t, err)

	err = fs.RemoveAll("link")
	require.NoError(t, err)
	_, err = os.Lstat(link)
	require.ErrorIs(t, err, os.ErrNotExist)
	requireFileContents(t, filepath.Join(target, "file"), []byte("target"))
}

func TestWindowsRemoveAllBackslashDotPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("backslash is only a path separator on windows")
	}

	t.Run("dot backslash file", func(t *testing.T) {
		dir := t.TempDir()
		fs, ok := newBoundOS(dir, true).(*BoundOS)
		require.True(t, ok)

		filename := filepath.Join(dir, "test-file")
		err := os.WriteFile(filename, []byte("anything"), 0o600)
		require.NoError(t, err)

		err = fs.RemoveAll(`.\test-file`)
		require.NoError(t, err)
		mustNotExist(filename)
	})

	for _, path := range windowsBackslashBaseDirAliases() {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			fs, ok := newBoundOS(dir, true).(*BoundOS)
			require.True(t, ok)

			err := fs.RemoveAll(path)
			require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRemoved)
			mustExist(dir)
		})
	}
}

func TestRemoveBaseDirAbsolutePath(t *testing.T) {
	tests := []struct {
		name   string
		remove func(fs *BoundOS, path string) error
	}{
		{
			name: "remove",
			remove: func(fs *BoundOS, path string) error {
				return fs.Remove(path)
			},
		},
		{
			name: "remove all",
			remove: func(fs *BoundOS, path string) error {
				return fs.RemoveAll(path)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs, ok := newBoundOS(dir, true).(*BoundOS)
			require.True(t, ok)

			err := tt.remove(fs, dir)
			require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRemoved)
			mustExist(dir)
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		elems  []string
		wanted string
	}{
		{
			elems:  []string{},
			wanted: "",
		},
		{
			elems:  []string{"/a", "b", "c"},
			wanted: filepath.FromSlash("/a/b/c"),
		},
		{
			elems:  []string{"/a", "b/c"},
			wanted: filepath.FromSlash("/a/b/c"),
		},
		{
			elems:  []string{"/a", ""},
			wanted: filepath.FromSlash("/a"),
		},
		{
			elems:  []string{"/a", "/", "b"},
			wanted: filepath.FromSlash("/a/b"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.wanted, func(t *testing.T) {
			assert := assert.New(t)
			fs := newBoundOS(t.TempDir(), true)

			got := fs.Join(tt.elems...)
			assert.Equal(tt.wanted, got)
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name            string
		cwd             string
		filename        string
		makeAbs         bool
		expected        string
		makeExpectedAbs bool
		wantErr         string
		deduplicatePath bool
		before          func(dir string)
	}{
		{
			name:     "path: same dir rel file",
			cwd:      "/working/dir",
			filename: "./file",
			expected: filepath.FromSlash("/working/dir/file"),
		},
		{
			name:     "path: descending rel file",
			cwd:      "/working/dir",
			filename: "file",
			expected: filepath.FromSlash("/working/dir/file"),
		},
		{
			name:     "path: ascending rel file 1",
			cwd:      "/working/dir",
			filename: "../file",
			expected: filepath.FromSlash("/working/dir/file"),
		},
		{
			name:     "path: ascending rel file 2",
			cwd:      "/working/dir",
			filename: "../../file",
			expected: filepath.FromSlash("/working/dir/file"),
		},
		{
			name:     "path: ascending rel file 3",
			cwd:      "/working/dir",
			filename: "/../../file",
			expected: filepath.FromSlash("/working/dir/file"),
		},
		{
			name:            "path: abs file within cwd",
			cwd:             filepath.FromSlash("/working/dir"),
			filename:        filepath.FromSlash("/working/dir/abs-file"),
			expected:        filepath.FromSlash("/working/dir/abs-file"),
			deduplicatePath: true,
		},
		{
			name:     "path: abs file within cwd wo deduplication",
			cwd:      filepath.FromSlash("/working/dir"),
			filename: filepath.FromSlash("/working/dir/abs-file"),
			expected: filepath.FromSlash("/working/dir/working/dir/abs-file"),
		},
		{
			name:     "path: abs file within cwd",
			cwd:      "/working/dir",
			filename: "/outside/dir/abs-file",
			expected: filepath.FromSlash("/working/dir/outside/dir/abs-file"),
		},
		{
			name:            "abs symlink: within cwd w abs descending target",
			filename:        "ln-cwd-cwd",
			makeAbs:         true,
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
				require.NoError(t, err)
			},
			deduplicatePath: true,
		},
		{
			name:            "abs symlink: within cwd w rel descending target",
			filename:        "ln-rel-cwd-cwd",
			makeAbs:         true,
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd"))
				require.NoError(t, err)
			},
			deduplicatePath: true,
		},
		{
			name:            "abs symlink: within cwd w abs ascending target",
			filename:        "ln-cwd-up",
			makeAbs:         true,
			expected:        "/some/outside/dir",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("/some/outside/dir", filepath.Join(dir, "ln-cwd-up"))
				require.NoError(t, err)
			},
			deduplicatePath: true,
		},
		{
			name:            "abs symlink: within cwd w rel ascending target",
			filename:        "ln-rel-cwd-up",
			makeAbs:         true,
			expected:        "outside-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("../../outside-cwd", filepath.Join(dir, "ln-rel-cwd-up"))
				require.NoError(t, err)
			},
			deduplicatePath: true,
		},
		{
			name:            "rel symlink: within cwd w abs descending target",
			filename:        "ln-cwd-cwd",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
				require.NoError(t, err)
			},
			deduplicatePath: true,
		},
		{
			name:            "rel symlink: within cwd w rel descending target",
			filename:        "ln-rel-cwd-cwd2",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd2"))
				require.NoError(t, err)
			},
		},
		{
			name:            "rel symlink: within cwd w abs ascending target",
			filename:        "ln-cwd-up2",
			expected:        "/outside/path/up",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("/outside/path/up", filepath.Join(dir, "ln-cwd-up2"))
				require.NoError(t, err)
			},
		},
		{
			name:            "rel symlink: within cwd w rel ascending target",
			filename:        "ln-rel-cwd-up2",
			expected:        "outside",
			makeExpectedAbs: true,
			before: func(dir string) {
				err := os.Symlink("../../../../outside", filepath.Join(dir, "ln-rel-cwd-up2"))
				require.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			cwd := tt.cwd
			if cwd == "" {
				cwd = t.TempDir()
			}

			fs, ok := newBoundOS(cwd, tt.deduplicatePath).(*BoundOS)
			assert.True(ok)

			if tt.before != nil {
				tt.before(cwd)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(cwd, filename)
			}

			expected := tt.expected
			if tt.makeExpectedAbs {
				expected = filepath.Join(cwd, expected)
			}

			got, err := fs.abs(filename)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(expected, got)
		})
	}
}

func TestAbsReturnsSecureJoinError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on windows")
	}

	dir := t.TempDir()
	fs, ok := newBoundOS(dir, true).(*BoundOS)
	require.True(t, ok)

	err := os.Symlink("loop", filepath.Join(dir, "loop"))
	require.NoError(t, err)

	got, err := fs.abs(filepath.Join("loop", "file"))
	require.Error(t, err)
	assert.Empty(t, got)
}

func TestReadDir(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	f, err := os.Create(filepath.Join(dir, "file1"))
	require.NoError(t, err)
	assert.NotNil(f)
	require.NoError(t, f.Close())

	f, err = os.Create(filepath.Join(dir, "file2"))
	require.NoError(t, err)
	assert.NotNil(f)
	require.NoError(t, f.Close())

	dirs, err := fs.ReadDir(dir)
	require.NoError(t, err)
	assert.NotNil(dirs)
	assert.Len(dirs, 2)

	dirs, err = fs.ReadDir(".")
	require.NoError(t, err)
	assert.NotNil(dirs)
	assert.Len(dirs, 2)

	err = os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
	require.NoError(t, err)
	dirs, err = fs.ReadDir("symlink")
	require.ErrorContains(t, err, notFoundError())
	assert.Nil(dirs)
}

func TestAbsNoFollowRootRelativePath(t *testing.T) {
	root := t.TempDir()
	fs, ok := newBoundOS(root, true).(*BoundOS)
	require.True(t, ok)

	got, err := fs.absNoFollow(filepath.Join("a", "..", "b", "file"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "b", "file"), got)
}

func TestMkdirAll(t *testing.T) {
	assert := assert.New(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	target := "abc"
	targetAbs := filepath.Join(cwd, target)
	fs := newBoundOS(cwd, true)

	// Even if CWD is changed outside of the fs instance,
	// the base dir must still be observed.
	err := os.Chdir(os.TempDir())
	require.NoError(t, err)

	err = fs.MkdirAll(target, 0o700)
	require.NoError(t, err)

	fi, err := os.Stat(targetAbs)
	require.NoError(t, err)
	assert.NotNil(fi)

	err = os.Mkdir(filepath.Join(root, "outside"), 0o700)
	require.NoError(t, err)
	err = os.Symlink(filepath.Join(root, "outside"), filepath.Join(cwd, "symlink"))
	require.NoError(t, err)

	err = fs.MkdirAll(filepath.Join(cwd, "symlink", "new-dir"), 0o700)
	require.NoError(t, err)

	// For windows, the volume name must be removed from the path or
	// it will lead to an invalid path.
	if vol := filepath.VolumeName(root); vol != "" {
		root = root[len(vol):]
	}

	mustExist(filepath.Join(cwd, root, "outside", "new-dir"))
}

func TestRename(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	oldFile := "old-file"
	newFile := filepath.Join("newdir", "newfile")

	// Even if CWD is changed outside of the fs instance,
	// the base dir must still be observed.
	err := os.Chdir(os.TempDir())
	require.NoError(t, err)

	f, err := fs.Create(oldFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	if runtime.GOOS != "windows" {
		resetUmask := umask(2)
		err = fs.Rename(oldFile, newFile)
		require.NoError(t, err)
		resetUmask()

		di, err := os.Stat(filepath.Dir(filepath.Join(dir, newFile)))
		require.NoError(t, err)
		assert.NotNil(di)
		expected := 0o775
		actual := int(di.Mode().Perm())
		assert.Equal(
			expected, actual, "Permission mismatch - expected: 0o%o, actual: 0o%o", expected, actual,
		)
	} else {
		err = fs.Rename(oldFile, newFile)
		require.NoError(t, err)
	}

	fi, err := os.Stat(filepath.Join(dir, newFile))
	require.NoError(t, err)
	assert.NotNil(fi)

	err = fs.Rename(filepath.FromSlash("/tmp/outside/cwd/file1"), newFile)
	require.ErrorIs(t, err, os.ErrNotExist)

	err = fs.Rename(oldFile, filepath.FromSlash("/tmp/outside/cwd/file2"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRenameRenamesSymlink(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir, true)
	target := filepath.Join(dir, "target-file")
	link := filepath.Join(dir, "link")
	renamed := filepath.Join(dir, "renamed-link")

	err := os.WriteFile(target, []byte("target"), 0o600)
	require.NoError(t, err)
	err = os.Symlink("target-file", link)
	require.NoError(t, err)

	err = fs.Rename("link", "renamed-link")
	require.NoError(t, err)
	_, err = os.Lstat(link)
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
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	for _, from := range []string{"", ".", "./", "/..", "/foo/..", dir} {
		err := fs.Rename(from, "renamed")
		require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRenamed)
		mustExist(dir)
	}
}

func TestWindowsRenameBackslashDotPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("backslash is only a path separator on windows")
	}

	t.Run("dot backslash file", func(t *testing.T) {
		dir := t.TempDir()
		fs := newBoundOS(dir, true)

		oldPath := filepath.Join(dir, "test-file")
		newPath := filepath.Join(dir, "renamed")
		err := os.WriteFile(oldPath, []byte("anything"), 0o600)
		require.NoError(t, err)

		err = fs.Rename(`.\test-file`, "renamed")
		require.NoError(t, err)
		mustNotExist(oldPath)
		mustExist(newPath)
	})

	for _, path := range windowsBackslashBaseDirAliases() {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			err := fs.Rename(path, "renamed")
			require.ErrorIs(t, err, billy.ErrBaseDirCannotBeRenamed)
			mustExist(dir)
		})
	}
}

func windowsBackslashBaseDirAliases() []string {
	return []string{`.\..`, `\..`, `\foo\..`}
}

func mustExist(filename string) {
	fi, err := os.Stat(filename)
	if err != nil || fi == nil {
		panic(fmt.Sprintf("file %s should exist", filename))
	}
}

func mustNotExist(filename string) {
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		panic(fmt.Sprintf("file %s should not exist", filename))
	}
}

func requireFileContents(t *testing.T, filename string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func notFoundError() string {
	switch runtime.GOOS {
	case "windows":
		return "The system cannot find the " // {path,file} specified
	default:
		return "no such file or directory"
	}
}
