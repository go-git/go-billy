//go:build js
// +build js

package osfs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/stretchr/testify/assert"
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

func TestNewAPI(t *testing.T) {
	_ = New("/")
}
