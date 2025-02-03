//go:build !wasm
// +build !wasm

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

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.ErrorContains(t, err, fmt.Sprint(dir, filepath.FromSlash("/above/cwd/prefix")))
	assert.Nil(f)

	tempDir := os.TempDir()
	// For windows, volume name must be removed.
	if v := filepath.VolumeName(tempDir); v != "" {
		tempDir = strings.TrimPrefix(tempDir, v)
	}

	f, err = fs.TempFile(tempDir, "prefix")
	require.ErrorContains(t, err, filepath.Join(dir, tempDir, "prefix"))
	assert.Nil(f)
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
			name:     "file: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  "path outside base dir",
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
			wantErr:  "path outside base dir",
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
			expected:        filepath.Join("cwd-target/file"),
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
			wantErr:  "path outside base dir",
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
			wantErr:  "path outside base dir",
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  "path outside base dir",
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  "path outside base dir",
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
			name:     "path: rel pointing outside cwd",
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
			name: "same dot dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir, true)
			},
			filename: ".",
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
			name: "rel path to file above cwd",
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
			name: "abs symlink: pointing outside is forced to descend",
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
			wantErr:  notFoundError(),
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
			name: "same dir file",
			before: func(t *testing.T, dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "same dir symlink",
			before: func(t *testing.T, dir string) billy.Filesystem {
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
			name: "rel path to file above cwd",
			before: func(t *testing.T, dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(t *testing.T, dir string) billy.Filesystem {
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

func TestInsideBaseDirEval(t *testing.T) {
	assert := assert.New(t)

	fs := BoundOS{baseDir: "/"}
	b, err := fs.insideBaseDirEval("a")
	assert.True(b)
	require.NoError(t, err)

	fs = BoundOS{baseDir: ""}
	b, err = fs.insideBaseDirEval(filepath.Join("a", "b", "c"))
	assert.True(b)
	require.NoError(t, err)
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

	err = fs.Rename(oldFile, newFile)
	require.NoError(t, err)

	fi, err := os.Stat(filepath.Join(dir, newFile))
	require.NoError(t, err)
	assert.NotNil(fi)

	err = fs.Rename(filepath.FromSlash("/tmp/outside/cwd/file1"), newFile)
	require.ErrorIs(t, err, os.ErrNotExist)

	err = fs.Rename(oldFile, filepath.FromSlash("/tmp/outside/cwd/file2"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func mustExist(filename string) {
	fi, err := os.Stat(filename)
	if err != nil || fi == nil {
		panic(fmt.Sprintf("file %s should exist", filename))
	}
}

func notFoundError() string {
	switch runtime.GOOS {
	case "windows":
		return "The system cannot find the " // {path,file} specified
	default:
		return "no such file or directory"
	}
}
