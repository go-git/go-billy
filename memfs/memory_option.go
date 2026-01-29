package memfs

type Option func(*options)

type options struct {
	umask uint32
}

// WithUmask sets the umask for the memfs filesystem. The umask controls the
// default permissions for newly created files and directories by clearing
// specified permission bits. If not set, defaults to 0o022.
func WithUmask(mask uint32) Option {
	return func(o *options) {
		o.umask = mask
	}
}
