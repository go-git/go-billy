package test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/helper/chroot"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func utilWriteFile(fs Basic, name string, data []byte, perm os.FileMode) error {
	return util.WriteFile(fs, name, data, perm)
}
func utilReadFile(fs Basic, name string) ([]byte, error) { return util.ReadFile(fs, name) }

func TestCopy_Basic(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		t.Helper()
		require.NoError(t, utilWriteFile(fs, "src", []byte("hello"), 0o644))
		require.NoError(t, util.Copy(fs, "src", fs, "dst"))
		got, err := utilReadFile(fs, "dst")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))
	})
}

func TestCopy_PreservesMode(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		t.Helper()
		require.NoError(t, utilWriteFile(fs, "src", []byte("x"), 0o755))
		require.NoError(t, util.Copy(fs, "src", fs, "dst"))
		fi, err := fs.Stat("dst")
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm())
	})
}

func TestCopy_OverwriteAndEmpty(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		t.Helper()
		require.NoError(t, utilWriteFile(fs, "dst", []byte("old"), 0o600))
		require.NoError(t, utilWriteFile(fs, "src", nil, 0o644))
		require.NoError(t, util.Copy(fs, "src", fs, "dst"))
		got, err := utilReadFile(fs, "dst")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestCopy_OsfsDispatch(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs not in allFS here")
	}
	tmp := t.TempDir()
	fs := osfs.New(tmp, osfs.WithBoundOS())
	require.NoError(t, util.WriteFile(fs, "src", []byte("osfs"), 0o644))
	require.NoError(t, util.Copy(fs, "src", fs, "dst"))
	got, err := util.ReadFile(fs, "dst")
	require.NoError(t, err)
	assert.Equal(t, "osfs", string(got))
}

func TestCopy_MemfsDispatch(t *testing.T) {
	fs := memfs.New()
	require.NoError(t, util.WriteFile(fs, "src", []byte("data"), 0o644))
	require.NoError(t, util.Copy(fs, "src", fs, "dst"))
	got, err := util.ReadFile(fs, "dst")
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))
	require.NoError(t, util.WriteFile(fs, "dst", []byte("changed"), 0o644))
	got, err = util.ReadFile(fs, "src")
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))
}

func TestCopy_MissingSrc(t *testing.T) {
	eachFS(t, func(t *testing.T, fs Filesystem) {
		t.Helper()
		err := util.Copy(fs, "nope", fs, "dst")
		assert.Error(t, err)
		_, err = fs.Stat("dst")
		assert.True(t, os.IsNotExist(err))
	})
}

// TestCopy_ChrootDispatch ensures util.Copy unwraps both source and
// destination chroot wrappers to reach the underlying osfs Copier. The
// result must be correct.
func TestCopy_ChrootDispatch(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs not available")
	}
	tmp := t.TempDir()
	inner := osfs.New(tmp, osfs.WithBoundOS())
	fs := chroot.New(inner, ".")

	require.NoError(t, utilWriteFile(fs, "src", []byte("chroot"), 0o644))
	require.NoError(t, util.Copy(fs, "src", fs, "dst"))
	got, err := utilReadFile(fs, "dst")
	require.NoError(t, err)
	assert.Equal(t, "chroot", string(got))
}

// TestCopy_ChrootSourceToBareDest ensures util.Copy unwraps a chroot-wrapped
// source so the osfs Copier can resolve it against a bare osfs destination.
func TestCopy_ChrootSourceToBareDest(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs not available")
	}
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.MkdirAll(dstDir, 0o755))

	srcWrapped := chroot.New(osfs.New(srcDir, osfs.WithBoundOS()), ".")
	dstBare := osfs.New(dstDir, osfs.WithBoundOS())

	require.NoError(t, utilWriteFile(srcWrapped, "f", []byte("wrapped-src"), 0o644))
	require.NoError(t, util.Copy(srcWrapped, "f", dstBare, "f"))
	got, err := os.ReadFile(filepath.Join(dstDir, "f"))
	require.NoError(t, err)
	assert.Equal(t, "wrapped-src", string(got))
}

// TestCopy_SymlinkDestinationContained ensures a destination path whose
// intermediate component is a symlink escaping the osfs root is not followed.
// The copy must fail and nothing must be written outside the root.
func TestCopy_SymlinkDestinationContained(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs/symlink not available")
	}
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "evil")))

	src := osfs.New(root, osfs.WithBoundOS())
	require.NoError(t, utilWriteFile(src, "payload", []byte("ESCAPED"), 0o644))

	err := util.Copy(src, "payload", src, "evil/pwned")
	assert.Error(t, err, "copy through escaping symlink must fail")
	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "no file must be created outside the root")
}

func TestCopy_RejectsSymlinkSource(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs/symlink not available")
	}
	root := t.TempDir()
	fs := osfs.New(root, osfs.WithBoundOS())
	require.NoError(t, utilWriteFile(fs, "realfile", []byte("payload"), 0o644))
	require.NoError(t, fs.Symlink("realfile", "link"))

	err := util.Copy(fs, "link", fs, "dst")
	require.Error(t, err)
	_, statErr := os.Stat(filepath.Join(root, "dst"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestCopy_RejectsSymlinkSource_MemfsDest(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs/symlink not available")
	}
	root := t.TempDir()
	srcOS := osfs.New(root, osfs.WithBoundOS())
	require.NoError(t, utilWriteFile(srcOS, "realfile", []byte("payload"), 0o644))
	require.NoError(t, srcOS.Symlink("realfile", "link"))

	memDst := memfs.New()
	err := util.Copy(srcOS, "link", memDst, "out")
	require.Error(t, err)
	_, statErr := memDst.Stat("out")
	assert.True(t, os.IsNotExist(statErr))
}

// TestCopy_DirectorySourceRejected ensures a directory source is not a
// regular file and is rejected without creating the destination.
func TestCopy_DirectorySourceRejected(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs not available")
	}
	root := t.TempDir()
	fs := osfs.New(root, osfs.WithBoundOS())
	require.NoError(t, os.Mkdir(filepath.Join(root, "srcdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "srcdir", "inside"), []byte("x"), 0o644))

	err := util.Copy(fs, "srcdir", fs, "dstDir")
	assert.Error(t, err, "directory source must be rejected")
	_, statErr := os.Stat(filepath.Join(root, "dstDir"))
	assert.True(t, os.IsNotExist(statErr), "destination must not be created for a directory source")
}

// TestCopy_DoesNotPreserveMtime ensures the destination mtime is set to
// the time of copy, not the source mtime, matching the util.Copy contract.
func TestCopy_DoesNotPreserveMtime(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		t.Skip("osfs not available")
	}
	root := t.TempDir()
	fs := osfs.New(root, osfs.WithBoundOS())
	require.NoError(t, utilWriteFile(fs, "src", []byte("mtime"), 0o644))

	srcFi, err := fs.Stat("src")
	require.NoError(t, err)
	oldMtime := srcFi.ModTime()
	require.NoError(t, os.Chtimes(filepath.Join(root, "src"), oldMtime.Add(-72*time.Hour), oldMtime.Add(-72*time.Hour)))

	require.NoError(t, util.Copy(fs, "src", fs, "dst"))
	dstFi, err := fs.Stat("dst")
	require.NoError(t, err)
	assert.True(t, dstFi.ModTime().After(srcFi.ModTime()), "destination mtime must not be copied from source")
}
