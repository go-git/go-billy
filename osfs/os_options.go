package osfs

type Option func(*options)

// WithBoundOS returns the option of using a Bound filesystem OS.
func WithBoundOS() Option {
	return func(o *options) {
		o.Type = BoundOSFS
	}
}

// WithChrootOS returns the option of using a Chroot filesystem OS.
func WithChrootOS() Option {
	return func(o *options) {
		o.Type = ChrootOSFS
	}
}

type options struct {
	Type
}

type Type int

const (
	ChrootOSFS Type = iota
	BoundOSFS
)
