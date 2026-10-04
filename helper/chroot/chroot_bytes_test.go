package chroot

import (
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/stretchr/testify/assert"
)

// bytesFile is a billy.BytesFile with a fixed result.
type bytesFile struct {
	test.FileMock
	data []byte
	ok   bool
}

func (f *bytesFile) Bytes() ([]byte, bool) { return f.data, f.ok }

// Ensure the wrapper keeps optional zero-copy access at compile-time.
var _ billy.BytesFile = (*file)(nil)

func TestFileBytesForwarding(t *testing.T) {
	t.Parallel()

	data := []byte("mapped")
	tests := []struct {
		name   string
		inner  billy.File
		want   []byte
		wantOK bool
	}{
		{name: "forwards available", inner: &bytesFile{data: data, ok: true}, want: data, wantOK: true},
		{name: "forwards unavailable", inner: &bytesFile{}, wantOK: false},
		{name: "inner without Bytes", inner: &test.FileMock{}, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &file{File: tc.inner, name: "x"}
			got, ok := f.Bytes()
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
