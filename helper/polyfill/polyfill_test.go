package polyfill

import (
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/stretchr/testify/assert"
)

var (
	helper = New(&test.BasicMock{})
)

func TestTempFile(t *testing.T) {
	_, err := helper.TempFile("", "")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestReadDir(t *testing.T) {
	_, err := helper.ReadDir("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestMkdirAll(t *testing.T) {
	err := helper.MkdirAll("", 0)
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestSymlink(t *testing.T) {
	err := helper.Symlink("", "")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestReadlink(t *testing.T) {
	_, err := helper.Readlink("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestLstat(t *testing.T) {
	_, err := helper.Lstat("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestChroot(t *testing.T) {
	_, err := helper.Chroot("")
	assert.ErrorIs(t, err, billy.ErrNotSupported)
}

func TestRoot(t *testing.T) {
	assert.Equal(t, string(filepath.Separator), helper.Root())
}

func TestCapabilities(t *testing.T) {
	testCapabilities(t, new(test.BasicMock))
	testCapabilities(t, new(test.OnlyReadCapFs))
	testCapabilities(t, new(test.NoLockCapFs))
}

func testCapabilities(t *testing.T, basic billy.Basic) {
	baseCapabilities := billy.Capabilities(basic)

	fs := New(basic)
	capabilities := billy.Capabilities(fs)
	assert.Equal(t, baseCapabilities, capabilities)
}
