//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package tnet

import (
	"net"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

func TestListen(t *testing.T) {
	tests := []struct {
		name     string
		network  string
		address  string
		expected bool
	}{
		{"tcp", "tcp", ":0", true},
		{"tcp4", "tcp4", "127.0.0.1:0", true},
		{"tcp6", "tcp6", "[::1]:0", true},
		{"udp", "udp", ":0", false},
		{"udp4", "udp4", "127.0.0.1:0", false},
		{"udp6", "udp6", "[::1]:0", false},
		{"unix", "unix", "/tmp/test.sock", false},
	}
	for _, test := range tests {
		if !netutil.TestableNetwork(test.network) {
			t.Logf("skipping %s test", test.name)
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			ln, err := Listen(test.network, test.address)
			defer func() {
				if err == nil {
					ln.Close()
				}
			}()
			require.Equal(t, test.expected, err == nil)
			if err == nil {
				assert.NotEqual(t, -1, ln.(*tcpListener).FD())
				assert.NotEmpty(t, ln.Addr())
			}
		})
	}
}

func TestListenerAccept(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address string
	}{
		{"tcp normal accept", "tcp", ":0"},
		{"tcp4 normal accept", "tcp4", "127.0.0.1:0"},
		{"tcp6 normal accept", "tcp6", "[::1]:0"},
	}
	for _, test := range tests {
		if !netutil.TestableNetwork(test.network) {
			t.Logf("skipping %s test", test.name)
			continue
		}
		t.Run(test.network, func(t *testing.T) {
			MassiveConnections = true
			DefaultCleanUpThrottle = 0
			ln, err := Listen(test.network, test.address)
			require.Nil(t, err)
			defer ln.Close()

			_, err = ln.Accept()
			assert.NotNil(t, err)
			ne, ok := err.(net.Error)
			require.Equal(t, true, ok)
			require.Equal(t, true, ne.Temporary())
			assert.Equal(t, false, ne.Timeout())

			client, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
			require.Nil(t, err)
			defer client.Close()

			conn, err := ln.Accept()
			c := conn.(*tcpconn)
			t.Logf("conn size: %v\n", unsafe.Sizeof(*c))
			t.Logf("c.writevData.IsNil(): %v\n", c.writevData.IsNil())
			t.Logf("IOData size: %v\n", unsafe.Sizeof(iovec.IOData{}))
			t.Logf("conn.writevData size: %v\n", unsafe.Sizeof(c.writevData))
			t.Logf("conn.postponeWrite size: %v\n", unsafe.Sizeof(c.postpone))

			t.Logf("alignOf: %v\n", unsafe.Alignof(c)) // 8
			assert.Nil(t, err)
			assert.NotNil(t, conn)
			assert.NotEmpty(t, conn.LocalAddr())
			assert.NotEmpty(t, conn.RemoteAddr())
		})
	}
}

func TestListenerAcceptAfterClose(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address string
	}{
		{"tcp close before accept", "tcp", ":0"},
		{"tcp4 close before accept", "tcp4", "127.0.0.1:0"},
		{"tcp6 close before accept", "tcp6", "[::1]:0"},
	}
	for _, test := range tests {
		if !netutil.TestableNetwork(test.network) {
			t.Logf("skipping %s test", test.name)
			continue
		}
		t.Run(test.network, func(t *testing.T) {
			ln, err := Listen(test.network, test.address)
			require.Nil(t, err)

			client, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
			require.Nil(t, err)
			defer client.Close()

			ln.Close()

			_, err = ln.Accept()
			assert.NotNil(t, err)
		})
	}
}
