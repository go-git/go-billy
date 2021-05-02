package util_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
)

func TestTempFile(t *testing.T) {
	fs := memfs.New()

	dir, err := util.TempDir(fs, "", "util_test")
	if err != nil {
		t.Fatal(err)
	}
	defer util.RemoveAll(fs, dir)

	f, err := util.TempFile(fs, dir, "foo")
	if f == nil || err != nil {
		t.Errorf("TempFile(%q, `foo`) = %v, %v", dir, f, err)
	}
}

func TestTempDir_WithDir(t *testing.T) {
	fs := memfs.New()

	dir := os.TempDir()
	name, err := util.TempDir(fs, dir, "util_test")
	if name == "" || err != nil {
		t.Errorf("TempDir(dir, `util_test`) = %v, %v", name, err)
	}
	if name != "" {
		util.RemoveAll(fs, name)
		re := regexp.MustCompile("^" + regexp.QuoteMeta(filepath.Join(dir, "util_test")) + "[0-9]+$")
		if !re.MatchString(name) {
			t.Errorf("TempDir(`"+dir+"`, `util_test`) created bad name %s", name)
		}
	}
}

func TestReadFile(t *testing.T) {
	fs := memfs.New()
	f, err := util.TempFile(fs, "", "")
	if err != nil {
		t.Fatal(err)
	}

	f.Write([]byte("foo"))
	f.Close()

	data, err := util.ReadFile(fs, f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "foo" || err != nil {
		t.Errorf("ReadFile(%q, %q) = %v, %v", fs, f.Name(), data, err)
	}

}

func TestTempDir(t *testing.T) {
	fs := memfs.New()
	f, err := util.TempDir(fs, "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = filepath.Rel(os.TempDir(), f)
	if err != nil {
		t.Errorf(`TempDir(fs, "", "") = %s, should be relative to os.TempDir if root filesystem`, f)
	}
}

func TestTempDir_WithNonRoot(t *testing.T) {
	fs := memfs.New()
	fs, _ = fs.Chroot("foo")
	f, err := util.TempDir(fs, "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = filepath.Rel(os.TempDir(), f)
	if err == nil {
		t.Errorf(`TempDir(fs, "", "") = %s, should not be relative to os.TempDir on not root filesystem`, f)
	}
}
