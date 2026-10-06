package util

import (
	"errors"
	"io"
	"os"

	"github.com/go-git/go-billy/v6"
)

var errNotRegular = errors.New("util: source is not a regular file")

// Copy copies the regular file at srcName in srcFS to dstName in dstFS,
// preserving the source mode. Symlink sources are rejected on both the Copier
// and Open+Create+io.Copy fallback paths. It unwraps both src and dst to their
// leaf filesystems, dispatches to a leaf Copier, or falls back to Open+Create+io.Copy.
func Copy(srcFS billy.Filesystem, srcName string, dstFS billy.Filesystem, dstName string) error {
	srcLeaf, srcLeafName := unwrap(srcFS, srcName)
	dstLeaf, dstLeafName := unwrap(dstFS, dstName)
	if copier, ok := dstLeaf.(billy.Copier); ok {
		return copier.CopyFrom(srcLeaf, srcLeafName, dstLeafName)
	}
	return copyFileFallback(srcLeaf, srcLeafName, dstLeaf, dstLeafName)
}

func unwrap(fs billy.Filesystem, name string) (billy.Filesystem, string) {
	for {
		switch t := fs.(type) {
		case billy.RootedWrapper:
			name = t.Underlying().Join(t.Root(), name)
			next := asFilesystem(t.Underlying())
			if next == nil {
				return fs, name
			}
			fs = next
		case billy.UnderlyingFS:
			next := asFilesystem(t.Underlying())
			if next == nil {
				return fs, name
			}
			fs = next
		default:
			return fs, name
		}
	}
}

func asFilesystem(b billy.Basic) billy.Filesystem {
	if f, ok := b.(billy.Filesystem); ok {
		return f
	}
	return nil
}

func copyFileFallback(src billy.Filesystem, srcName string, dst billy.Filesystem, dstName string) (err error) {
	srcFi, err := src.Lstat(srcName)
	if err != nil {
		return err
	}
	if !srcFi.Mode().IsRegular() {
		return errNotRegular
	}
	in, err := src.Open(srcName)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := dst.OpenFile(dstName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcFi.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}
