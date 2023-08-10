package poller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_descCache(t *testing.T) {
	dc := &descCache{
		cache: make([]*Desc, 0, 16),
	}
	d := dc.alloc()
	require.NotNil(t, d)
	d.FD = 1
	dc.markFree(d)
	require.Equal(t, 1, d.FD)
	dc.free()
	require.Zero(t, d.FD)
}
