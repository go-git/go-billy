//go:build windows

package osfs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChrootPathKeepsDriveAbsolutePathFromEmptyBase(t *testing.T) {
	fs := newTestBoundOS(t, "")

	want := filepath.Clean(`C:\Users\runner\repo.git`)
	assert.Equal(t, want, fs.chrootPath(`C:\Users\runner\repo.git`))
	assert.Equal(t, want, fs.chrootPath(`/C:/Users/runner/repo.git`))
}
