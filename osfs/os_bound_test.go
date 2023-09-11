//go:build !js
// +build !js

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

	"github.com/go-git/go-billy/v5"
	"github.com/onsi/gomega"
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
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "file: rel path to above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "file: rel path to below cwd",
			before: func(dir string) billy.Filesystem {
				os.Mkdir(filepath.Join(dir, "sub"), 0o700)
				os.WriteFile(filepath.Join(dir, "sub/rel-below-cwd"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "sub/rel-below-cwd",
		},
		{
			name: "file: abs inside cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
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
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "symlink: rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../outside/cwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
		{
			name: "symlink: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
				g.Expect(fi).To(gomega.BeNil())
			} else {
				g.Expect(err).To(gomega.BeNil())
				g.Expect(fi).ToNot(gomega.BeNil())
				g.Expect(fi.Close()).To(gomega.Succeed())
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
				os.Mkdir(filepath.Join(dir, "new-dir"), 0o701)
				return newBoundOS(dir, true)
			},
			target: filepath.FromSlash("../../../some/random/path"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			// Even if CWD is changed outside of the fs instance,
			// the base dir must still be observed.
			err := os.Chdir(os.TempDir())
			g.Expect(err).ToNot(gomega.HaveOccurred())

			link := filepath.Join(dir, tt.link)

			diBefore, _ := os.Lstat(filepath.Dir(link))

			err = fs.Symlink(tt.target, tt.link)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			fi, err := os.Lstat(link)
			if tt.wantStatErr != "" {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantStatErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(fi).ToNot(gomega.BeNil())
			}

			got, err := os.Readlink(link)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(got).To(gomega.Equal(tt.target))

			diAfter, err := os.Lstat(filepath.Dir(link))
			g.Expect(err).ToNot(gomega.HaveOccurred())

			if diBefore != nil {
				g.Expect(diAfter.Mode()).To(gomega.Equal(diBefore.Mode()))
			}
		})
	}
}

func TestTempFile(t *testing.T) {
	g := gomega.NewWithT(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	f, err := fs.TempFile("", "prefix")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(f).ToNot(gomega.BeNil())
	g.Expect(f.Name()).To(gomega.ContainSubstring(os.TempDir()))
	g.Expect(f.Close()).ToNot(gomega.HaveOccurred())

	f, err = fs.TempFile("/above/cwd", "prefix")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring(fmt.Sprint(dir, filepath.FromSlash("/above/cwd/prefix"))))
	g.Expect(f).To(gomega.BeNil())

	tempDir := os.TempDir()
	// For windows, volume name must be removed.
	if v := filepath.VolumeName(tempDir); v != "" {
		tempDir = strings.TrimPrefix(tempDir, v)
	}

	f, err = fs.TempFile(tempDir, "prefix")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring(filepath.Join(dir, tempDir, "prefix")))
	g.Expect(f).To(gomega.BeNil())
}

func TestChroot(t *testing.T) {
	g := gomega.NewWithT(t)
	tmp := t.TempDir()
	fs := newBoundOS(tmp, true)

	f, err := fs.Chroot("test")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(f).ToNot(gomega.BeNil())
	g.Expect(f.Root()).To(gomega.Equal(filepath.Join(tmp, "test")))
}

func TestRoot(t *testing.T) {
	g := gomega.NewWithT(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	root := fs.Root()
	g.Expect(root).To(gomega.Equal(dir))
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
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
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
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
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

				os.Mkdir(cwd, 0o700)
				os.Mkdir(outside, 0o700)

				os.Symlink(outside, filepath.Join(cwd, "symlink"))
				os.WriteFile(filepath.Join(outside, "file"), []byte("anything"), 0o600)

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

				os.MkdirAll(cwdTarget, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(cwdTarget, cwdAlt)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				return newBoundOS(cwd, true)
			},
			filename:        "symlink-file",
			expected:        filepath.Join("cwd-target/file"),
			makeExpectedAbs: true,
		},
		{
			name: "symlink: outside cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				os.Mkdir(cwdTarget, 0o700)
				os.Mkdir(outsideDir, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(outsideDir, outside)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
				return newBoundOS(cwd, true)
			},
			filename: "symlink-outside/symlink-file",
			wantErr:  "path outside base dir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
				g.Expect(got).To(gomega.BeEmpty())
			} else {
				g.Expect(err).To(gomega.BeNil())
				g.Expect(got).To(gomega.Equal(expected))
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
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
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

				os.MkdirAll(cwdTarget, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(cwdTarget, cwdAlt)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				return newBoundOS(cwd, true)
			},
			filename: "symlink-file",
			makeAbs:  false,
		},
		{
			name: "symlink: outside cwd + baseDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				os.Mkdir(cwdTarget, 0o700)
				os.Mkdir(outsideDir, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(outsideDir, outside)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
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
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "file: abs",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
				g.Expect(fi).To(gomega.BeNil())
			} else {
				g.Expect(err).To(gomega.BeNil())
				g.Expect(fi).ToNot(gomega.BeNil())
				g.Expect(fi.Name()).To(gomega.Equal(filepath.Base(tt.filename)))
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
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			wantErr:  notFoundError(),
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			wantErr:  notFoundError(),
		},

		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  notFoundError(),
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
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
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
				g.Expect(fi).To(gomega.BeNil())
			} else {
				g.Expect(err).To(gomega.BeNil())
				g.Expect(fi).ToNot(gomega.BeNil())
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
			name: "same dir file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "symlink: same dir",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
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

				os.Mkdir(cwd, 0o700)
				os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				os.WriteFile(outsideFile, []byte("anything"), 0o600)
				os.Symlink(outsideFile, filepath.Join(cwd, "remove-abs-symlink"))
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

				os.Mkdir(cwd, 0o700)
				os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				os.WriteFile(outsideFile, []byte("anything"), 0o600)
				os.Symlink(filepath.Join("..", "outside-cwd", "file2"), filepath.Join(cwd, "remove-abs-symlink2"))
				return newBoundOS(cwd, true)
			},
			filename: "remove-rel-symlink",
			wantErr:  notFoundError(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).To(gomega.BeNil())
			}
		})
	}
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "parent with children",
			before: func(dir string) billy.Filesystem {
				os.MkdirAll(filepath.Join(dir, "parent/children"), 0o600)
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
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "test-file",
		},
		{
			name: "same dir symlink",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				return newBoundOS(dir, true)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return newBoundOS(dir, true)
			},
			filename: "symlink",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			dir := t.TempDir()
			fs := newBoundOS(dir, true).(*BoundOS)

			if tt.before != nil {
				fs = tt.before(dir).(*BoundOS)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.RemoveAll(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).To(gomega.BeNil())
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
			g := gomega.NewWithT(t)
			fs := newBoundOS(t.TempDir(), true)

			got := fs.Join(tt.elems...)
			g.Expect(got).To(gomega.Equal(tt.wanted))
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
				os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
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
				os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd"))
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
				os.Symlink("/some/outside/dir", filepath.Join(dir, "ln-cwd-up"))
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
				os.Symlink("../../outside-cwd", filepath.Join(dir, "ln-rel-cwd-up"))
			},
			deduplicatePath: true,
		},
		{
			name:            "rel symlink: within cwd w abs descending target",
			filename:        "ln-cwd-cwd",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
			},
			deduplicatePath: true,
		},
		{
			name:            "rel symlink: within cwd w rel descending target",
			filename:        "ln-rel-cwd-cwd2",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd2"))
			},
		},
		{
			name:            "rel symlink: within cwd w abs ascending target",
			filename:        "ln-cwd-up2",
			expected:        "/outside/path/up",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("/outside/path/up", filepath.Join(dir, "ln-cwd-up2"))
			},
		},
		{
			name:            "rel symlink: within cwd w rel ascending target",
			filename:        "ln-rel-cwd-up2",
			expected:        "outside",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("../../../../outside", filepath.Join(dir, "ln-rel-cwd-up2"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			cwd := tt.cwd
			if cwd == "" {
				cwd = t.TempDir()
			}

			fs := newBoundOS(cwd, tt.deduplicatePath).(*BoundOS)
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
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}

			g.Expect(got).To(gomega.Equal(expected))
		})
	}
}

func TestReadDir(t *testing.T) {
	g := gomega.NewWithT(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	f, err := os.Create(filepath.Join(dir, "file1"))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(f).ToNot(gomega.BeNil())
	g.Expect(f.Close()).To(gomega.Succeed())

	f, err = os.Create(filepath.Join(dir, "file2"))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(f).ToNot(gomega.BeNil())
	g.Expect(f.Close()).To(gomega.Succeed())

	dirs, err := fs.ReadDir(dir)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(dirs).ToNot(gomega.BeNil())
	g.Expect(dirs).To(gomega.HaveLen(2))

	dirs, err = fs.ReadDir(".")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(dirs).ToNot(gomega.BeNil())
	g.Expect(dirs).To(gomega.HaveLen(2))

	os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
	dirs, err = fs.ReadDir("symlink")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring(notFoundError()))
	g.Expect(dirs).To(gomega.BeNil())
}

func TestMkdirAll(t *testing.T) {
	g := gomega.NewWithT(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	target := "abc"
	targetAbs := filepath.Join(cwd, target)
	fs := newBoundOS(cwd, true)

	// Even if CWD is changed outside of the fs instance,
	// the base dir must still be observed.
	err := os.Chdir(os.TempDir())
	g.Expect(err).ToNot(gomega.HaveOccurred())

	err = fs.MkdirAll(target, 0o700)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	fi, err := os.Stat(targetAbs)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(fi).ToNot(gomega.BeNil())

	err = os.Mkdir(filepath.Join(root, "outside"), 0o700)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	err = os.Symlink(filepath.Join(root, "outside"), filepath.Join(cwd, "symlink"))
	g.Expect(err).ToNot(gomega.HaveOccurred())

	err = fs.MkdirAll(filepath.Join(cwd, "symlink", "new-dir"), 0o700)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	// For windows, the volume name must be removed from the path or
	// it will lead to an invalid path.
	if vol := filepath.VolumeName(root); vol != "" {
		root = root[len(vol):]
	}

	mustExist(filepath.Join(cwd, root, "outside", "new-dir"))
}

func TestRename(t *testing.T) {
	g := gomega.NewWithT(t)
	dir := t.TempDir()
	fs := newBoundOS(dir, true)

	oldFile := "old-file"
	newFile := filepath.Join("newdir", "newfile")

	// Even if CWD is changed outside of the fs instance,
	// the base dir must still be observed.
	err := os.Chdir(os.TempDir())
	g.Expect(err).ToNot(gomega.HaveOccurred())

	f, err := fs.Create(oldFile)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(f.Close()).To(gomega.Succeed())

	err = fs.Rename(oldFile, newFile)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	fi, err := os.Stat(filepath.Join(dir, newFile))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(fi).ToNot(gomega.BeNil())

	err = fs.Rename(filepath.FromSlash("/tmp/outside/cwd/file1"), newFile)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring(notFoundError()))

	err = fs.Rename(oldFile, filepath.FromSlash("/tmp/outside/cwd/file2"))
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring(notFoundError()))
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
