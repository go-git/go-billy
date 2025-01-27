package memfs

type Option func(*options)

type options struct {
	newMutex func() mutex
}

// WithoutMutex disables thread safety. This is a temporary option
// for users to opt-out from the default setting.
func WithoutMutex() Option {
	return func(o *options) {
		o.newMutex = newNoOpMutex
	}
}
