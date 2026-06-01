//go:build !wasm

package osfs

import (
	"io/fs"
	"reflect"
	"testing"
)

func TestReadDirFilesystem(t *testing.T) {
	fs := New(t.TempDir())

	for _, name := range []string{"file1.txt", "file3.txt", "file2.txt"} {
		f, err := fs.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	entries, err := fs.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	want := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, e := range entries {
		if e.Name() != want[i] {
			t.Fatalf("expected %s at index %d, got %s", want[i], i, e.Name())
		}
	}
}

var _ fs.File = &file{}

func TestDefault(t *testing.T) {
	want := &BoundOS{}
	got := Default

	if reflect.TypeOf(got) != reflect.TypeFor[*BoundOS]() {
		t.Errorf("wanted Default to be %T got %T", want, got)
	}
}

var (
	// API call assertions
	_ = New("/")
	_ = New("/", WithBoundOS())
)
