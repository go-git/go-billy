//go:build wasip1

package osfs

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/stretchr/testify/assert"
)

func TestOpenDoesNotCreateDir(t *testing.T) {
	fs := New("/some/path")
	_, err := fs.Open("dir/non-existent")
	assert.Error(t, err)

	_, err = fs.Stat(filepath.Join("/some/path", "dir"))
	assert.Error(t, err)
}

func TestCapabilities(t *testing.T) {
	fs := New("/some/path")
	_, ok := fs.(billy.Capable)
	assert.True(t, ok)

	caps := billy.Capabilities(fs)
	assert.Equal(t, billy.DefaultCapabilities, caps)
}

func TestDefault(t *testing.T) {
	want := Default
	got := Default

	if reflect.TypeOf(got) != reflect.TypeOf(want) {
		t.Errorf("wanted Default to be %T got %T", want, got)
	}
}

func TestNewAPI(t *testing.T) {
	_ = New("/")
}
