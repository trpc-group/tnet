// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

//go:build linux || freebsd || dragonfly || darwin
// +build linux freebsd dragonfly darwin

package netutil_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

func newLocalListener(network string) (ln net.Listener, err error) {
	switch network {
	case "tcp":
		return net.Listen("tcp", ":0")
	case "tcp4":
		return net.Listen("tcp4", "127.0.0.1:0")
	case "tcp6":
		return net.Listen("tcp6", "[::1]:0")
	default:
		return nil, fmt.Errorf("%s is not support", network)
	}
}

func newLocalPackListener(network string) (ln net.PacketConn, err error) {
	switch network {
	case "udp":
		return net.ListenPacket("udp", ":0")
	case "udp4":
		return net.ListenPacket("udp4", "127.0.0.1:0")
	case "udp6":
		return net.ListenPacket("udp6", "[::1]:0")
	default:
		return nil, fmt.Errorf("%s is not support", network)
	}
}

func TestGetDupTCPFD(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6"} {
		if !netutil.TestableNetwork(network) {
			t.Logf("skipping %s test", network)
			continue
		}
		t.Run(network, func(t *testing.T) {
			ln, err := newLocalListener(network)
			require.Nil(t, err)
			defer ln.Close()

			fd0, err := netutil.GetFD(ln)
			assert.Nil(t, err)
			fd1, err := netutil.DupFD(ln)
			assert.Nil(t, err)
			defer func() {
				unix.Close(fd1)
			}()
			require.NotEmpty(t, fd1)
			require.NotEqual(t, fd0, fd1)

			conn, err := net.Dial(network, ln.Addr().String())
			require.Nil(t, err)
			defer conn.Close()
			fd2, err := netutil.GetFD(conn)
			assert.Nil(t, err)
			fd3, err := netutil.DupFD(conn)
			defer func() {
				unix.Close(fd3)
			}()
			assert.Nil(t, err)
			require.NotEmpty(t, fd3)
			require.NotEqual(t, fd2, fd3)
		})
	}
}

func TestGetDupUDPFd(t *testing.T) {
	for _, network := range []string{"udp", "udp4", "udp6"} {
		if !netutil.TestableNetwork(network) {
			t.Logf("skipping %s test", network)
			continue
		}
		t.Run(network, func(t *testing.T) {
			ln, err := newLocalPackListener(network)
			require.Nil(t, err)
			defer ln.Close()
			fd0, err := netutil.GetFD(ln)
			assert.Nil(t, err)
			fd1, err := netutil.DupFD(ln)
			assert.Nil(t, err)
			defer func() {
				unix.Close(fd1)
			}()
			require.NotEmpty(t, fd1)
			require.NotEqual(t, fd0, fd1)

			conn, err := net.Dial(network, ln.LocalAddr().String())
			require.Nil(t, err)
			defer conn.Close()
			fd2, err := netutil.GetFD(conn)
			assert.Nil(t, err)
			fd3, err := netutil.DupFD(conn)
			defer func() {
				unix.Close(fd3)
			}()
			assert.Nil(t, err)
			require.NotEmpty(t, fd3)
			require.NotEqual(t, fd2, fd3)
		})
	}
}

func TestGetDupFdNotSupport(t *testing.T) {
	ln, err := net.Listen("unix", "/tmp/test.sock")
	require.Nil(t, err)
	defer ln.Close()
	_, err = netutil.GetFD(ln)
	assert.Nil(t, err)

	_, err = netutil.DupFD(ln)
	assert.NotNil(t, err)
}

func TestGetDupFdAfterClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	ln.Close()
	_, err = netutil.GetFD(ln)
	assert.NotNil(t, err)

	_, err = netutil.DupFD(ln)
	assert.NotNil(t, err)
}
