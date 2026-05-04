//go:build wasip1

package osfs

import "github.com/go-git/go-billy/v6"

func boundCapabilities() billy.Capability {
	return billy.DefaultCapabilities
}
