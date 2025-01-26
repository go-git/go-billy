package temporal

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempFileDefaultPath(t *testing.T) {
	fs := New(memfs.New(), "foo")
	f, err := fs.TempFile("", "bar")

	require.NoError(t, err)
	require.NoError(t, f.Close())

	assert.True(t, strings.HasPrefix(f.Name(), fs.Join("foo", "bar")))
}
