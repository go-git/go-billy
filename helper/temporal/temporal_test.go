package temporal

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

func TestTempFileDefaultPath(t *testing.T) {
	fs := New(memfs.New(), "foo")
	f, err := fs.TempFile("", "bar")

	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	assert.True(t, strings.HasPrefix(f.Name(), fs.Join("foo", "bar")))
}
