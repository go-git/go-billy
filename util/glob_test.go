package util_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	fs := memfs.New()
	err := util.WriteFile(fs, "foo/qux", nil, 0o644)
	require.NoError(t, err)
	err = util.WriteFile(fs, "foo/bar", nil, 0o644)
	require.NoError(t, err)
	err = util.WriteFile(fs, "foo/baz/foo", nil, 0o644)
	require.NoError(t, err)

	names, err := util.Glob(fs, "*/b*")
	assert := assert.New(t)
	require.NoError(t, err)
	assert.Len(names, 2)

	sort.Strings(names)

	assert.Equal([]string{
		filepath.Join("foo", "bar"),
		filepath.Join("foo", "baz"),
	}, names)
}
