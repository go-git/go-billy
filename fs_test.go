package billy_test

import (
	"testing"

	. "github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestCapabilities(t *testing.T) {
	cases := []struct {
		caps     Capability
		expected bool
	}{
		{LockCapability, false},
		{ReadCapability, true},
		{ReadCapability | WriteCapability, true},
		{ReadCapability | WriteCapability | ReadAndWriteCapability | TruncateCapability, true},
		{ReadCapability | WriteCapability | ReadAndWriteCapability | TruncateCapability | LockCapability, false},
		{TruncateCapability | LockCapability, false},
	}

	// This filesystem supports all capabilities except for LockCapability
	fs := new(test.NoLockCapFs)

	for _, e := range cases {
		assert.Equal(t, CapabilityCheck(fs, e.caps), e.expected)
	}

	dummy := new(test.BasicMock)
	assert.Equal(t, Capabilities(dummy), DefaultCapabilities)
}
