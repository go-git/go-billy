//go:build !js

package test

import (
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/osfs"
)

// Compile-time check that osfs implements Copier. Gated to !js because the
// osfs Copier and *RootOS are !js-only.
var (
	_ billy.Copier = (*osfs.BoundOS)(nil)
	_ billy.Copier = (*osfs.RootOS)(nil)
)
