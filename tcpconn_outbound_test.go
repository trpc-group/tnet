package tnet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/metrics"
)

type nopSockCloser struct{}

func (nopSockCloser) Close() error {
	return nil
}

func TestTCPConnWritevOutboundBufferLimitExceeded(t *testing.T) {
	tc := &tcpconn{
		readTrigger:         make(chan struct{}),
		closedFinished:      make(chan struct{}),
		outboundBufferLimit: 5,
		nfd: netFD{
			sock: nopSockCloser{},
		},
	}
	tc.inBuffer.Initialize()
	tc.outBuffer.Initialize()
	tc.outBuffer.Writev(false, []byte("abcde"))
	require.Equal(t, 5, OutboundBuffered(tc))

	before := metrics.Get(metrics.TCPOutboundBufferLimitExceeded)
	n, err := tc.Writev([]byte("f"))
	require.True(t, errors.Is(err, ErrOutboundBufferLimitExceeded))
	require.Zero(t, n)
	require.False(t, tc.IsActive())
	require.Equal(t, before+1, metrics.Get(metrics.TCPOutboundBufferLimitExceeded))
}

func TestTCPConnWriteToOutboundBufferNil(t *testing.T) {
	var tc *tcpconn
	n, err := tc.writeToOutboundBuffer([]byte("a"))
	require.True(t, errors.Is(err, ErrConnClosed))
	require.Zero(t, n)
}
