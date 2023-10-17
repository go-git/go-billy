//go:build wasip1
// +build wasip1

package test

import "os"

var (
	customMode            os.FileMode = 0600
	expectedSymlinkTarget             = "/dir/file"
)
