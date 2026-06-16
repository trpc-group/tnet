package tls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerOutboundBufferLimitOption(t *testing.T) {
	opts := &serverOptions{}
	WithServerOutboundBufferLimit(1024)(opts)
	assert.Equal(t, 1024, opts.outboundBufferLimit)
	WithServerOutboundBufferLimit(1024)(nil)
}
