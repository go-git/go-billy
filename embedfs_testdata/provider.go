// Package embedfs_testdata provides embedded test data for billy embedfs testing.
// This package is only imported by test code and won't be included in production builds.
package embedfs_testdata

import (
	"embed"
)

//go:embed testdata
var TestData embed.FS

// GetTestData returns the raw embed.FS for tests to wrap with their own embedfs.New().
// This avoids import cycles while providing embedded test data.
func GetTestData() *embed.FS {
	return &TestData
}