//go:build !js

package osfs

type Option func(*options)

// WithMmap opts the filesystem returned by [New] (and any [RootOS]
// returned by [FromRoot]) into an mmap-backed implementation of
// [billy.File] for read-only opens on platforms that support it
// (currently darwin and linux). On other platforms the option is
// accepted but has no effect.
//
// EXPERIMENTAL: this option's semantics may change. Today every
// read-only [BoundOS.Open] / [RootOS.Open] (and any [BoundOS.OpenFile]
// / [RootOS.OpenFile] without write flags) returns a mmap-backed
// handle when the option is set. That includes small files and
// sequential-read workloads where mmap can be neutral or net-
// negative compared to plain file I/O. If we find a meaningful
// reason to make the opt-in finer-grained (per-call rather than
// per-FS), the option's effect on [Open]/[OpenFile] will narrow
// accordingly.
//
// mmap also changes failure semantics: truncating the underlying
// file while a read is in flight raises SIGBUS instead of returning
// an error, and replacing the file via rename leaves the mapping
// pointing at the old inode. Callers that may see the file mutate
// underneath them should leave the option off.
//
// The mmap-backed file is read-only by construction; it does not
// satisfy [billy.Syncer] (Sync is meaningless on a read-only
// mapping) or the [Locker] interface even though the surrounding
// filesystem advertises both capabilities. Code that type-asserts
// against those interfaces on a handle returned by an open with
// [WithMmap] active should be prepared for the assertion to fail.
func WithMmap() Option {
	return func(o *options) {
		o.mmap = true
	}
}
