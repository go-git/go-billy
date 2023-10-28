//go:build !windows && !wasip1
// +build !windows,!wasip1

package test

import "os"

var (
	customMode            os.FileMode = 0755
	expectedSymlinkTarget             = "/dir/file"
)
