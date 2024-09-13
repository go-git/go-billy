package iofs

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	billyfs "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

type errorList interface {
	Unwrap() []error
}

type wrappedError interface {
	Unwrap() error
}

// TestWithFSTest leverages the packaged Go fstest package, which seems comprehensive
func TestWithFSTest(t *testing.T) {
	t.Parallel()
	memfs := memfs.New()
	iofs := Wrap(memfs)

	files := map[string]string{
		"foo.txt":     "hello, world",
		"bar.txt":     "goodbye, world",
		"dir/baz.txt": "こんにちわ, world",
	}
	created_files := make([]string, 0, len(files))
	for filename, contents := range files {
		makeFile(memfs, t, filename, contents)
		created_files = append(created_files, filename)
	}

	err := fstest.TestFS(iofs, created_files...)
	if err != nil {
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			err = unwrapped
		}
		if errs, ok := err.(errorList); ok {
			for _, e := range errs.Unwrap() {

				if strings.Contains(e.Error(), "ModTime") {
					// Memfs returns the current time for Stat().ModTime(), which triggers
					// a diff complaint in fstest.  We can ignore this, or store modtimes
					// for every file in Memfs (at a cost of 16 bytes / file).
					t.Log("Skipping ModTime error (ok).")
				} else {
					t.Errorf("Unexpected fstest error: %v", e)
				}
			}
		} else {
			t.Fatalf("Failed to test fs:\n%v", err)
		}
	}
}

func TestDeletes(t *testing.T) {
	t.Parallel()
	memfs := memfs.New()
	iofs := Wrap(memfs).(fs.ReadFileFS)

	makeFile(memfs, t, "foo.txt", "hello, world")
	makeFile(memfs, t, "deleted", "nothing to see")

	if _, err := iofs.ReadFile("nonexistent"); err == nil {
		t.Errorf("expected error for nonexistent file")
	}

	data, err := iofs.ReadFile("deleted")
	if err != nil {
		t.Fatalf("failed to read file before delete: %v", err)
	}
	if string(data) != "nothing to see" {
		t.Errorf("unexpected contents before delete: %v", data)
	}

	if err := memfs.Remove("deleted"); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}

	if _, err = iofs.ReadFile("deleted"); err == nil {
		t.Errorf("file existed after delete!")
	}
}

func makeFile(fs billyfs.Basic, t *testing.T, filename string, contents string) {
	t.Helper()
	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("failed to create file %s: %v", filename, err)
	}
	defer file.Close()
	_, err = file.Write([]byte(contents))
	if err != nil {
		t.Fatalf("failed to write to file %s: %v", filename, err)
	}
}
