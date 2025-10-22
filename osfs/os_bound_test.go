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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundOSCapabilities(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)
	c, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := c.Capabilities()
	assert.Equal(t, billy.DefaultCapabilities&billy.SyncCapability, caps)
}

func TestFromRoot(t *testing.T) {
	validRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { validRoot.Close() })

	closedRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, closedRoot.Close())

	tests := []struct {
		name     string
		root     *os.Root
		wantRoot string
		wantErr  string
	}{
		{
			name:     "valid root",
			root:     validRoot,
			wantRoot: validRoot.Name(),
		},
		{
			name:    "nil root",
			root:    nil,
			wantErr: "root is nil",
		},
		{
			name:     "closed root",
			root:     closedRoot,
			wantRoot: closedRoot.Name(),
			wantErr:  "file already closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			fs := FromRoot(tt.root)
			require.NotNil(t, fs)
			assert.IsType(&BoundOS{}, fs)
			assert.Equal(tt.wantRoot, fs.Root())

			if tt.wantErr != "" {
				_, err := fs.Stat(".")
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			f, err := fs.Create("test-file")
			require.NoError(t, err)
			require.NoError(t, f.Close())

			_, err = fs.Stat("test-file")
			require.NoError(t, err)
		})
	}
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
				return newBoundOS(dir)
			},
			filename: "test-file",
		},
		{
			name: "file: dot rel same dir",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "./test-file",
		},
		{
			name: "file: rel path to above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "../../rel-above-cwd",
			wantErr:  ErrPathEscapesParent.Error(),
		},
		{
			name: "file: rel path to below cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Mkdir(filepath.Join(dir, "sub"), 0o700)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "sub/rel-below-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "sub/rel-below-cwd",
		},
		{
			name: "file: abs inside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "file: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir)
			},
			filename: "/some/path/outside/cwd",
			wantErr:  ErrPathEscapesParent.Error(),
		},
		{
			name: "symlink: same dir abs",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(target, filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "symlink: same dir rel",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink("target-file", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "symlink: symlink to symlink",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				err := os.WriteFile(target, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink("target-file", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				err = os.Symlink("symlink", filepath.Join(dir, "symlink2"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink2",
		},
		{
			name: "symlink: rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../outside/cwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  ErrPathEscapesParent.Error(),
		},
		{
			name: "symlink: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  ErrPathEscapesParent.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir)

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
		makeAbs     bool
		before      func(dir string) billy.Filesystem
		wantStatErr string
	}{
		{
			name:   "link to abs valid target",
			link:   "symlink",
			target: filepath.FromSlash("/etc/passwd"),
		},
		{
			name:    "abs link to abs valid target",
			link:    "symlink",
			target:  filepath.FromSlash("/etc/passwd"),
			makeAbs: true,
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
			target: filepath.FromSlash("/etc/passwd"),
		},
		{
			name: "keep dir filemode if exists",
			link: "new-dir/symlink",
			before: func(dir string) billy.Filesystem {
				err := os.Mkdir(filepath.Join(dir, "new-dir"), 0o701)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			target: filepath.FromSlash("../../../some/random/path"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			// Even if CWD is changed outside of the fs instance,
			// the base dir must still be observed.
			err := os.Chdir(os.TempDir())
			require.NoError(t, err)

			link := filepath.Join(dir, tt.link)

			diBefore, _ := os.Lstat(filepath.Dir(link))

			lnk := tt.link
			if tt.makeAbs {
				lnk = link
			}

			err = fs.Symlink(tt.target, lnk)
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
	fs := newBoundOS(dir)

	// No dir provided means bound dir + `/.tmp`.
	f, err := fs.TempFile("", "prefix")
	require.NoError(t, err)
	assert.NotNil(f)
	prefix := filepath.Join(".tmp", "prefix")
	assert.True(strings.HasPrefix(f.Name(), filepath.Join(dir, prefix)), f.Name(), prefix)
	require.NoError(t, f.Close())

	f, err = fs.TempFile("/above/cwd", "prefix")
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(f)

	dir = os.TempDir()
	f, err = fs.TempFile("../../../above/cwd", "prefix")
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(f)

	dir = filepath.Join(dir, "/tmp")
	// For windows, volume name must be removed.
	if v := filepath.VolumeName(dir); v != "" {
		dir = strings.TrimPrefix(dir, v)
	}

	f, err = fs.TempFile(dir, "prefix")
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(f)
}

func TestChroot(t *testing.T) {
	assert := assert.New(t)
	tmp := t.TempDir()
	fs := newBoundOS(tmp)

	f, err := fs.Chroot("test")
	require.NoError(t, err)
	assert.NotNil(f)
	assert.Equal(filepath.Join(tmp, "test"), f.Root())
	assert.IsType(&BoundOS{}, f)
}

func TestRoot(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir)

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
		wantErr         error
	}{
		{
			name: "symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			expected: filepath.FromSlash("/etc/passwd"),
		},
		{
			name:     "file: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "symlink: abs symlink pointing outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			expected: filepath.FromSlash("/etc/passwd"),
			wantErr:  ErrPathEscapesParent,
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

				return newBoundOS(cwd)
			},
			filename: "current-dir/symlink/file",
			makeAbs:  true,
			wantErr:  ErrPathEscapesParent,
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
				return newBoundOS(cwd)
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
				return newBoundOS(cwd)
			},
			filename: "symlink-outside/symlink-file",
			wantErr:  ErrPathEscapesParent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir)

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
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
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
		wantErr  error
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
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
				return newBoundOS(cwd)
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

				return newBoundOS(cwd)
			},
			filename: "symlink-outside/symlink-file",
			makeAbs:  false,
			wantErr:  ErrPathEscapesParent,
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "file: rel",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "test-file",
		},
		{
			name: "file: dot rel",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "./test-file",
		},
		{
			name: "file: abs",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}
			fi, err := fs.Lstat(filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
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
		wantErr  error
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			wantErr:  ErrPathEscapesParent,
		},

		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				err := os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  false,
			wantErr:  ErrPathEscapesParent,
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "rel file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "test-file",
		},
		{
			name: "rel dot dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir)
			},
			filename: ".",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs := newBoundOS(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			fi, err := fs.Stat(filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
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
		after    func(t *testing.T, dir string)
		wantErr  error
	}{
		{
			name:     "path: rel pointing outside cwd w forward slash",
			filename: "/some/path/outside/cwd",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name:     "path: rel pointing outside cwd",
			filename: "../../../../path/outside/cwd",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "inexistent dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir)
			},
			filename: "inexistent",
			wantErr:  os.ErrNotExist,
		},
		{
			name: "same dot dir",
			before: func(dir string) billy.Filesystem {
				return newBoundOS(dir)
			},
			filename: ".",
			wantErr:  billy.ErrBaseDirCannotBeRemoved,
		},
		{
			name: "same dir file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
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
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file below cwd",
			before: func(dir string) billy.Filesystem {
				p := filepath.Join(dir, "sub")
				err := os.MkdirAll(p, 0o777)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(dir, "sub", "rel-below-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "./sub/rel-below-cwd",
		},
		{
			name: "rel path to file above cwd",
			before: func(dir string) billy.Filesystem {
				p := filepath.Join(dir, "sub")
				err := os.MkdirAll(p, 0o777)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(p)
			},
			filename: "../rel-above-cwd",
			wantErr:  ErrPathEscapesParent,
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing outside is deleted",
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
				return newBoundOS(cwd)
			},
			after: func(t *testing.T, dir string) {
				t.Helper()
				_, err := os.Stat(filepath.Join(dir, "outside-cwd/file"))
				require.NoError(t, err)
			},
			filename: "remove-abs-symlink",
		},
		{
			name: "rel symlink: pointing outside is deleted",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outsideFile := filepath.Join(dir, "outside-cwd", "file")

				err := os.Mkdir(cwd, 0o700)
				require.NoError(t, err)
				err = os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				require.NoError(t, err)
				err = os.WriteFile(outsideFile, []byte("anything"), 0o600)
				require.NoError(t, err)
				err = os.Symlink(filepath.Join("..", "outside-cwd", "file"), filepath.Join(cwd, "remove-rel-symlink"))
				require.NoError(t, err)
				return newBoundOS(cwd)
			},
			after: func(t *testing.T, dir string) {
				t.Helper()
				_, err := os.Stat(filepath.Join(dir, "outside-cwd/file"))
				require.NoError(t, err)
			},
			filename: "remove-rel-symlink",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs := newBoundOS(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.Remove(filename)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.after != nil {
				tt.after(t, dir)
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
				t.Helper()
				err := os.MkdirAll(filepath.Join(dir, "parent", "children"), 0o700)
				require.NoError(t, err)
				return newBoundOS(dir)
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
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
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
				return newBoundOS(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file above cwd",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
			},
			filename: "../../rel-above-cwd",
			wantErr:  ErrPathEscapesParent.Error(),
		},
		{
			name: "abs file",
			before: func(t *testing.T, dir string) billy.Filesystem {
				t.Helper()
				err := os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				require.NoError(t, err)
				return newBoundOS(dir)
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
				return newBoundOS(dir)
			},
			filename: "symlink",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dir := t.TempDir()
			fs, ok := newBoundOS(dir).(*BoundOS)
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
			fs := newBoundOS(t.TempDir())

			got := fs.Join(tt.elems...)
			assert.Equal(tt.wanted, got)
		})
	}
}

func TestReadDir(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	fs := newBoundOS(dir)

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
	require.ErrorIs(t, err, ErrPathEscapesParent)
	assert.Nil(dirs)
}

func TestMkdirAll(t *testing.T) {
	assert := assert.New(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")

	err := os.MkdirAll(cwd, 0o700)
	require.NoError(t, err)

	target := "abc"
	targetAbs := filepath.Join(cwd, target)
	fs := newBoundOS(cwd)

	// Even if CWD is changed outside of the fs instance,
	// the base dir must still be observed.
	err = os.Chdir(os.TempDir())
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
	require.ErrorIs(t, err, ErrPathEscapesParent)

	// For windows, the volume name must be removed from the path or
	// it will lead to an invalid path.
	if vol := filepath.VolumeName(root); vol != "" {
		root = root[len(vol):]
	}

	fi, _ = os.Stat(filepath.Join(root, "outside", "new-dir"))
	require.Nil(t, fi, "dir must not be created")
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	fs := newBoundOS(dir)

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
		assert.NotNil(t, di)
		expected := 0o775
		actual := int(di.Mode().Perm())
		assert.Equal(t, expected, actual,
			"Permission mismatch - expected: 0o%o, actual: 0o%o", expected, actual)
	} else {
		err = fs.Rename(oldFile, newFile)
		require.NoError(t, err)
	}

	fi, err := os.Stat(filepath.Join(dir, newFile))
	require.NoError(t, err)
	assert.NotNil(t, fi)

	err = fs.Rename(filepath.FromSlash("/tmp/outside/cwd/file1"), newFile)
	assert.ErrorIs(t, err, ErrPathEscapesParent)

	err = fs.Rename(oldFile, filepath.FromSlash("/tmp/outside/cwd/file2"))
	assert.ErrorIs(t, err, ErrPathEscapesParent)
}
