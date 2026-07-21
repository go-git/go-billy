//go:build !js

package osfs

import (
	"errors"
	"io"
	"os"

	"github.com/go-git/go-billy/v6"
)

var errNotRegular = errors.New("osfs: source is not a regular file")

func (fs *RootOS) CopyFrom(src billy.Filesystem, srcName, dstName string) error {
	return copyFileFrom(fs, src, srcName, dstName)
}

func (fs *BoundOS) CopyFrom(src billy.Filesystem, srcName, dstName string) error {
	return copyFileFrom(fs, src, srcName, dstName)
}

type containedOpener interface {
	openOSFile(name string, flag int, perm os.FileMode) (*os.File, error)
}

func copyFileFrom(dst billy.Filesystem, src billy.Filesystem, srcName, dstName string) (err error) {
	srcFi, err := src.Lstat(srcName)
	if err != nil {
		return err
	}
	if !srcFi.Mode().IsRegular() {
		return errNotRegular
	}
	perm := srcFi.Mode().Perm()

	dstOS, dstOK := dst.(containedOpener)
	srcOS, srcOK := src.(containedOpener)
	if dstOK && srcOK {
		in, err := srcOS.openOSFile(srcName, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := dstOS.openOSFile(dstName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}()
		return copyFileAccel(in, out)
	}

	in, err := src.Open(srcName)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := dst.OpenFile(dstName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
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

func (fs *RootOS) openOSFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	rel := fs.toRelative(name)
	if flag&os.O_CREATE != 0 {
		if err := createDir(fs.root, rel); err != nil {
			return nil, translateError(err, rel)
		}
	}
	openName := rel
	if openName == "" {
		openName = "."
	}
	f, err := fs.root.OpenFile(openName, flag, perm)
	if err != nil {
		return nil, translateError(err, rel)
	}
	return f, nil
}

func (fs *BoundOS) openOSFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	rfs, cleanup, err := fs.rootFSWithCreate(name, flag&os.O_CREATE != 0)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rfs.openOSFile(name, flag, perm)
}
