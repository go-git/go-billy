package iofs

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
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

// TestWithFSTest leverages the packaged Go fstest package, which seems comprehensive.
func TestWithFSTest(t *testing.T) {
	t.Parallel()
	memfs := memfs.New()
	iofs := New(memfs)

	files := map[string]string{
		"foo.txt":                       "hello, world",
		"bar.txt":                       "goodbye, world",
		filepath.Join("dir", "baz.txt"): "こんにちわ, world",
	}
	created_files := make([]string, 0, len(files))
	for filename, contents := range files {
		makeFile(memfs, t, filename, contents)
		created_files = append(created_files, filename)
	}

	if runtime.GOOS == "windows" {
		t.Skip("fstest.TestFS is not yet windows path aware")
	}

	err := fstest.TestFS(iofs, created_files...)
	if err != nil {
		checkFsTestError(t, err, files)
	}
}

func TestDeletes(t *testing.T) {
	t.Parallel()
	memfs := memfs.New()
	iofs := New(memfs).(fs.ReadFileFS)

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

func checkFsTestError(t *testing.T, err error, files map[string]string) {
	t.Helper()

	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		err = unwrapped
	}

	// Go >= 1.23 (after https://cs.opensource.google/go/go/+/74cce866f865c3188a34309e4ebc7a5c9ed0683d)
	// has nicely-Joined wrapped errors.  Try that first.
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
		if runtime.Version() >= "go1.23" {
			t.Fatalf("Failed to test fs:\n%v", err)
		}
		// filter lines from the error text corresponding to the above errors;
		// output looks like:
		// TestFS found errors:
		//         bar.txt: mismatch:
		//		entry.Info() = bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.377023639 +0000 UTC m=+0.002625548
		//		file.Stat() = bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.376907011 +0000 UTC m=+0.002508970
		//
		//	bar.txt: fs.Stat(...) = bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.381356651 +0000 UTC m=+0.006959191
		//		want bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.376907011 +0000 UTC m=+0.002508970
		//	bar.txt: fsys.Stat(...) = bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.381488617 +0000 UTC m=+0.007090346
		//		want bar.txt IsDir=false Mode=-rw-rw-rw- Size=14 ModTime=2024-09-17 10:09:00.376907011 +0000 UTC m=+0.002508970
		// We filter on "empty line" or "ModTime" or "$filename: mismatch" to ignore these.
		lines := strings.Split(err.Error(), "\n")
		filtered := make([]string, 0, len(lines))
		filename_mismatches := make(map[string]struct{}, len(files)*2)
		for name := range files {
			for dirname := name; dirname != "."; dirname = filepath.Dir(dirname) {
				filename_mismatches[dirname+": mismatch:"] = struct{}{}
			}
		}
		if strings.TrimSpace(lines[0]) == "TestFS found errors:" {
			lines = lines[1:]
		}
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.Contains(trimmed, "ModTime=") {
				continue
			}

			if _, ok := filename_mismatches[trimmed]; ok {
				continue
			}
			filtered = append(filtered, line)
		}
		if len(filtered) > 0 {
			t.Fatalf("Failed to test fs:\n%s", strings.Join(filtered, "\n"))
		}
	}
}
