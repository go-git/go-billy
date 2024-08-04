package util_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	fs := memfs.New()
	util.WriteFile(fs, "foo/qux", nil, 0o644)
	util.WriteFile(fs, "foo/bar", nil, 0o644)
	util.WriteFile(fs, "foo/baz/foo", nil, 0o644)

	names, err := util.Glob(fs, "*/b*")
	assert := assert.New(t)
	assert.NoError(err)
	assert.Len(names, 2)

	sort.Strings(names)

	assert.Equal(names, []string{
		filepath.Join("foo", "bar"),
		filepath.Join("foo", "baz"),
	})
}
