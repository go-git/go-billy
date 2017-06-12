package billy

import (
	"errors"
	"io"
	"os"
	"time"
)

var (
	ErrClosed       = errors.New("writing on closed file")
	ErrReadOnly     = errors.New("read-only filesystem")
	ErrNotSupported = errors.New("feature not supported")
)

// Filesystem abstract the operations in a storage-agnostic interface.
// Each method implementation mimics the behavior of the equivalent functions
// at the os package from the standard library.
type Filesystem interface {
	Basic
	Dir
	Symlink
	TempFile

	// Dir returns a new Filesystem from the same type of fs using as baseDir the
	// given path
	Dir(path string) Filesystem
	// Base returns the base path of the filesystem
	Base() string
}

// Basic abstract the basic operations in a storage-agnostic interface as
// an extension to the Basic interface
type Basic interface {
	// Create creates the named file with mode 0666 (before umask), truncating
	// it if it already exists. If successful, methods on the returned File can
	// be used for I/O; the associated file descriptor has mode O_RDWR. If there
	// is an error, it will be of type *PathError.
	Create(filename string) (File, error)
	// Open opens the named file for reading. If successful, methods on the
	// returned file can be used for reading; the associated file descriptor has
	// mode O_RDONLY. If there is an error, it will be of type *PathError.
	Open(filename string) (File, error)
	// OpenFile is the generalized open call; most users will use Open or Create
	// instead. It opens the named file with specified flag (O_RDONLY etc.) and
	// perm, (0666 etc.) if applicable. If successful, methods on the returned
	// File can be used for I/O. If there is an error, it will be of type
	// *PathError.
	OpenFile(filename string, flag int, perm os.FileMode) (File, error)
	// Stat returns a FileInfo describing the named file. If there is an error,
	// it will be of type *PathError.
	Stat(filename string) (FileInfo, error)
	// Rename renames (moves) oldpath to newpath. If newpath already exists and
	// is not a directory, Rename replaces it. OS-specific restrictions may
	// apply when oldpath and newpath are in different directories. If there is
	// an error, it will be of type *LinkError.
	Rename(oldpath, newpath string) error
	// Remove removes the named file or directory. If there is an error, it will
	// be of type *PathError.
	Remove(filename string) error
	// Join joins any number of path elements into a single path, adding a
	// Separator if necessary. Join calls filepath.Clean on the result; in
	// particular, all empty strings are ignored. On Windows, the result is a
	// UNC path if and only if the first path element is a UNC path.
	Join(elem ...string) string
}

type TempFile interface {
	// TempDir returns the default directory to use for temporary files.
	//TempDir() string
	TempFile(dir, prefix string) (File, error)
}

// Dir abstract the dir related operations in a storage-agnostic interface as
// an extension to the Basic interface.
type Dir interface {
	ReadDir(path string) ([]FileInfo, error)
	// MkdirAll creates a directory named path, along with any necessary
	// parents, and returns nil, or else returns an error. The permission bits
	// perm are used for all directories that MkdirAll creates. If path is/
	// already a directory, MkdirAll does nothing and returns nil.
	MkdirAll(filename string, perm os.FileMode) error
}

// Symlink abstract the symlink related operations in a storage-agnostic
// interface as an extension to the Basic interface.
type Symlink interface {
	// Lstat returns a FileInfo describing the named file. If the file is a
	// symbolic link, the returned FileInfo describes the symbolic link. Lstat
	// makes no attempt to follow the link. If there is an error, it will be of
	// type *PathError.
	Lstat(filename string) (FileInfo, error)
	// Symlink creates a symbolic-link from link to target. target may be an
	// absolute or relative path, and need not refer to an existing node.
	// Parent directories of link are created as necessary. If there is an
	// error, it will be of type *LinkError.
	Symlink(target, link string) error
	// Readlink returns the target path of link. If there is an error, it will
	// be of type *PathError.
	Readlink(link string) (string, error)
}

// Change abstract the FileInfo change related operations in a storage-agnostic
// interface as an extension to the Basic interface
type Change interface {
	// Chmod changes the mode of the named file to mode. If the file is a
	// symbolic link, it changes the mode of the link's target. If there is an
	// error, it will be of type *PathError.
	Chmod(name string, mode os.FileMode) error
	// Lchown changes the numeric uid and gid of the named file. If the file is
	// a symbolic link, it changes the uid and gid of the link itself. If there
	// is an error, it will be of type *PathError.
	Lchown(name string, uid, gid int) error
	// Chown changes the numeric uid and gid of the named file. If the file is a
	// symbolic link, it changes the uid and gid of the link's target. If there
	// is an error, it will be of type *PathError.
	Chown(name string, uid, gid int) error
	// Chtimes changes the access and modification times of the named file,
	// similar to the Unix utime() or utimes() functions.
	//
	// The underlying filesystem may truncate or round the values to a less
	// precise time unit. If there is an error, it will be of type *PathError.
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

// File implements io.Closer, io.Reader, io.Seeker, and io.Writer>
// Provides method to obtain the file name and the state of the file (open or closed).
type File interface {
	Filename() string
	IsClosed() bool
	io.Writer
	io.Reader
	io.Seeker
	io.Closer
}

type FileInfo os.FileInfo

type BaseFile struct {
	BaseFilename string
	Closed       bool
}

func (f *BaseFile) Filename() string {
	return f.BaseFilename
}

func (f *BaseFile) IsClosed() bool {
	return f.Closed
}

type removerAll interface {
	RemoveAll(string) error
}
