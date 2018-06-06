package billy_test

import (
	"testing"

	. "gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"

	. "gopkg.in/check.v1"
)

type FSSuite struct{}

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&FSSuite{})

func (s *FSSuite) TestCapabilities(c *C) {
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
	mem := memfs.New()

	for _, e := range cases {
		c.Assert(CapabilityCheck(mem, e.caps), Equals, e.expected)
	}
}
