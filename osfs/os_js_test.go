//go:build js

package osfs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/helper/chroot"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDoesNotCreateDir(t *testing.T) {
	path := t.TempDir()
	fs := New(path)
	_, err := fs.Open("dir/non-existent")
	assert.Error(t, err)

	_, err = fs.Stat(filepath.Join(path, "dir"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCapabilities(t *testing.T) {
	fs := New(t.TempDir())
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := billy.Capabilities(fs)
	assert.Equal(t, billy.DefaultCapabilities&^billy.LockCapability, caps)
}

func TestDefault(t *testing.T) {
	want := &chroot.ChrootHelper{} // memfs is wrapped around ChrootHelper.
	got := Default

	if reflect.TypeOf(got) != reflect.TypeOf(want) {
		t.Errorf("wanted Default to be %T got %T", want, got)
	}
}

func TestWithBoundOSReturnsBoundOS(t *testing.T) {
	got := New(t.TempDir(), WithBoundOS())
	assert.IsType(t, &BoundOS{}, got)
}

func TestWithChrootOSReturnsChrootHelper(t *testing.T) {
	got := New(t.TempDir(), WithChrootOS())
	assert.IsType(t, &chroot.ChrootHelper{}, got)
}

func TestDefaultTypeIsChrootOSFS(t *testing.T) {
	got := New(t.TempDir())
	assert.IsType(t, &chroot.ChrootHelper{}, got)
}

func TestBoundOSRoot(t *testing.T) {
	dir := t.TempDir()
	fs := New(dir, WithBoundOS()).(*BoundOS)
	assert.Equal(t, filepath.Join("/", dir), fs.Root())
}

func TestBoundOSReadWrite(t *testing.T) {
	fs := New(t.TempDir(), WithBoundOS())

	err := util.WriteFile(fs, "hello.txt", []byte("world"), 0o600)
	require.NoError(t, err)

	got, err := util.ReadFile(fs, "hello.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), got)
}

func TestBoundOSChrootScopesPaths(t *testing.T) {
	fs := New(t.TempDir(), WithBoundOS())

	sub, err := fs.Chroot("sub")
	require.NoError(t, err)
	assert.IsType(t, &BoundOS{}, sub)

	err = util.WriteFile(sub, "in-sub.txt", []byte("scoped"), 0o600)
	require.NoError(t, err)

	got, err := util.ReadFile(fs, filepath.Join("sub", "in-sub.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("scoped"), got)
}

func TestBoundOSRejectsParentTraversal(t *testing.T) {
	fs := New(t.TempDir(), WithBoundOS()).(*BoundOS)
	_, err := fs.Chroot("..")
	assert.ErrorIs(t, err, billy.ErrCrossedBoundary)
}

func TestWithDeduplicatePathIsAccepted(t *testing.T) {
	// WithDeduplicatePath has no effect on the in-memory js/wasm
	// implementation, but must remain callable for API parity with the
	// non-js build so go-git and other consumers compile unchanged.
	fs := New(t.TempDir(), WithBoundOS(), WithDeduplicatePath(false))
	assert.IsType(t, &BoundOS{}, fs)
}

// API call assertions. These ensure the exported option constructors remain
// available under GOOS=js so downstream code (e.g. go-git) keeps compiling.
var _ = New("/")
var _ = New("/", WithBoundOS())
var _ = New("/", WithChrootOS())
var _ = New("/", WithDeduplicatePath(false))

// Type constants must stay exported for any consumer that switches on them.
var (
	_ Type = ChrootOSFS
	_ Type = BoundOSFS
)
