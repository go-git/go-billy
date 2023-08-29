//go:build !js
// +build !js

package osfs

import (
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	want := &ChrootOS{}
	got := Default

	if reflect.TypeOf(got) != reflect.TypeOf(want) {
		t.Errorf("wanted Default to be %T got %T", want, got)
	}
}

func TestNewAPI(t *testing.T) {
	_ = New("/")
	_ = New("/", WithBoundOS())
	_ = New("/", WithChrootOS())
}
