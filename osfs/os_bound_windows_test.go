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

func TestNewBoundOSNormalizesSlashPrefixedDriveBase(t *testing.T) {
	fs := newTestBoundOS(t, `/C:/Users/runner/repo.git`)

	assert.Equal(t, filepath.Clean(`C:\Users\runner\repo.git`), fs.Root())
}

func TestOperationRootUsesDriveForAbsolutePathFromEmptyBase(t *testing.T) {
	fs := newTestBoundOS(t, "")

	assert.Equal(t, `C:\`, fs.operationRoot(`C:\Users\runner\.gitconfig`))
	assert.Equal(t, `C:\`, fs.operationRoot(`/C:/Users/runner/.gitconfig`))
}

func TestSlashPrefixedDrivePathIsCleanedUnderDriveRoot(t *testing.T) {
	assert.Equal(t, filepath.Join("Users", "runner", ".gitconfig"), cleanUnderRoot(`/C:/Users/runner/.gitconfig`))

	rel, ok := relativeInsideBase(`C:\`, `/C:/Users/runner/.gitconfig`)
	assert.True(t, ok)
	assert.Equal(t, filepath.Join("Users", "runner", ".gitconfig"), rel)
}
