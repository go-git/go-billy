package billy_test

import (
	"testing"

	. "github.com/go-git/go-billy/v6" //nolint
	"github.com/go-git/go-billy/v6/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMmapCapabilityIsDistinctBit(t *testing.T) {
	// MmapCapability must be its own bit, not overlapping any existing one,
	// and must NOT be part of DefaultCapabilities (it is platform/config gated).
	all := WriteCapability | ReadCapability | ReadAndWriteCapability |
		SeekCapability | TruncateCapability | LockCapability |
		SyncCapability
	require.Zero(t, all&MmapCapability, "MmapCapability overlaps an existing bit")
	require.Zero(t, DefaultCapabilities&MmapCapability,
		"MmapCapability must not be a default capability")
}

func TestMmapInterfaceShape(t *testing.T) {
	var _ Mmap = mmapStub{}
}

type mmapStub struct{}

func (mmapStub) Bytes() []byte                  { return nil }
func (mmapStub) Slice(off, length int64) []byte { return nil }
