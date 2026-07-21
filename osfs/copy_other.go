//go:build !linux && !js

package osfs

import (
	"io"
	"os"
)

func copyFileAccel(in, out *os.File) error {
	_, err := io.Copy(out, in)
	return err
}
