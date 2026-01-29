//go:build !wasm

package osfs

import (
	"io/fs"
	"reflect"
	"testing"
)

var _ fs.File = &file{}

func TestDefault(t *testing.T) {
	want := &ChrootOS{}
	got := Default

	if reflect.TypeOf(got) != reflect.TypeOf(want) {
		t.Errorf("wanted Default to be %T got %T", want, got)
	}
}

var (
	// API call assertions
	_ = New("/")
	_ = New("/", WithBoundOS())
	_ = New("/", WithChrootOS())
)
