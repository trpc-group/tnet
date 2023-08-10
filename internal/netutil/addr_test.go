// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Copyright (c) 2019 Andy Pan
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build linux || freebsd || dragonfly || darwin
// +build linux freebsd dragonfly darwin

package netutil_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

func TestSockaddrToTCPAndUDPAddr(t *testing.T) {
	tests := []struct {
		sa      unix.Sockaddr
		network string
		want    string
	}{
		{
			network: "tcp4",
			want:    "127.0.0.1:8080",
			sa: &unix.SockaddrInet4{
				Port: 8080,
				Addr: [4]byte{127, 0, 0, 1},
			},
		},
		{
			network: "tcp6",
			want:    "[2001:4860:0:2001::68]:9090",
			sa: &unix.SockaddrInet6{
				Port:   9090,
				ZoneId: 0,
				Addr:   [16]byte{0x20, 0x01, 0x48, 0x60, 0, 0, 0x20, 0x01, 0, 0, 0, 0, 0, 0, 0x00, 0x68},
			},
		},
		{
			network: "tcp6",
			want:    "[::1%100]:9091",
			sa: &unix.SockaddrInet6{
				Port:   9091,
				ZoneId: 100,
				Addr:   [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
		},
	}
	for _, tt := range tests {
		if !netutil.TestableNetwork(tt.network) {
			t.Logf("skipping %s test", tt.want)
			continue
		}
		t.Run(tt.want, func(t *testing.T) {
			tcpAddr := netutil.SockaddrToTCPOrUnixAddr(tt.sa)
			assert.Equal(t, "tcp", tcpAddr.Network())
			assert.Equal(t, tt.want, tcpAddr.String())

			udpAddr := netutil.SockaddrToUDPAddr(tt.sa)
			assert.Equal(t, "udp", udpAddr.Network())
			assert.Equal(t, tt.want, udpAddr.String())
		})
	}
}

func TestSockaddrToUnixAddr(t *testing.T) {
	file := "/tmp/test.sock"
	sa := &unix.SockaddrUnix{
		Name: file,
	}

	addr := netutil.SockaddrToTCPOrUnixAddr(sa)
	assert.Equal(t, "unix", addr.Network())
	assert.Equal(t, file, addr.String())
}

func TestSockaddrToTCPAddrWithIPv6Zone(t *testing.T) {
	if !netutil.TestableNetwork("tcp6") {
		t.Logf("skipping %s test", "TestSockaddrToTCPAddrWithIPv6Zone")
		return
	}

	sa := &unix.SockaddrInet6{
		Port: 9090,
		Addr: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	addr := netutil.SockaddrToTCPOrUnixAddr(sa)
	assert.Equal(t, "tcp", addr.Network())
	assert.Equal(t, "[::1]:9090", addr.String())
}

func TestConvertEndian(t *testing.T) {
	assert.Equal(t, uint16(10607), netutil.LittleToBigEndian(28457))
	assert.Equal(t, uint16(28457), netutil.BigToLittleEndian(10607))
}

func TestUDPAddrToSockaddrSlice(t *testing.T) {
	addr4, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:51624")
	expected4 := []byte{2, 0, 201, 168, 127, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	addr, err := netutil.UDPAddrToSockaddrSlice(addr4)
	assert.Nil(t, err)
	assert.Equal(t, expected4, addr)

	addr6, _ := net.ResolveUDPAddr("udp6", "[::1]:42356")
	family := unix.AF_INET6
	expected6 := []byte{byte(family), 0, 165, 116, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}
	addr, err = netutil.UDPAddrToSockaddrSlice(addr6)
	assert.Nil(t, err)
	assert.Equal(t, expected6, addr)

}

func TestSockaddrSliceToUDPAddr(t *testing.T) {
	sockaddr4 := []byte{2, 0, 201, 168, 127, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	expected4, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:51624")
	addr, err := netutil.SockaddrSliceToUDPAddr(sockaddr4)
	assert.Nil(t, err)
	assert.Equal(t, expected4.String(), addr.String())

	family := unix.AF_INET6
	sockaddr6 := []byte{byte(family), 0, 165, 116, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}
	expected6, _ := net.ResolveUDPAddr("udp6", "[::1]:42356")
	addr, err = netutil.SockaddrSliceToUDPAddr(sockaddr6)
	assert.Nil(t, err)
	assert.Equal(t, expected6.String(), addr.String())
}

func TestSockaddrSliceToUDPAddr_Error(t *testing.T) {
	invalidAddr := make([]byte, netutil.SockaddrSize+1)
	addr, err := netutil.SockaddrSliceToUDPAddr(invalidAddr)
	assert.NotNil(t, err)
	assert.Nil(t, addr)

	invalidAddr = []byte{3, 0, 201, 168, 127, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	addr, err = netutil.SockaddrSliceToUDPAddr(invalidAddr)
	assert.NotNil(t, err)
	assert.Nil(t, addr)
}

func TestAddrToSockAddr(t *testing.T) {
	addr4, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:51624")
	sa, err := netutil.AddrToSockAddr(addr4)
	assert.Nil(t, err)
	sa4, ok := sa.(*unix.SockaddrInet4)
	assert.Equal(t, true, ok)
	assert.Equal(t, 51624, sa4.Port)
	assert.Equal(t, [4]byte{127, 0, 0, 1}, sa4.Addr)

	addr6, _ := net.ResolveTCPAddr("tcp6", "[2001:4860:0:2001::68]:9090")
	sa, err = netutil.AddrToSockAddr(addr6)
	assert.Nil(t, err)
	sa6, ok := sa.(*unix.SockaddrInet6)
	assert.Equal(t, true, ok)
	assert.Equal(t, 9090, sa6.Port)
	assert.Equal(t, [16]byte{0x20, 0x01, 0x48, 0x60, 0, 0, 0x20, 0x01, 0, 0, 0, 0, 0, 0, 0x00, 0x68}, sa6.Addr)

	addrIP, _ := net.ResolveIPAddr("IP", "127.0.0.1:51624")
	_, err = netutil.AddrToSockAddr(addrIP)
	assert.NotNil(t, err)
}

func getUnixSockaddr(network, address string) (unix.Sockaddr, error) {
	addr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}
	sa, err := netutil.AddrToSockAddr(addr)
	if err != nil {
		return nil, err
	}
	return sa, nil
}

func TestUnixSockaddrToSockaddrSlice(t *testing.T) {
	sa := make([]byte, netutil.SockaddrSize)
	unixsa, err := getUnixSockaddr("tcp4", "127.0.0.1:12345")
	assert.Nil(t, err)
	err = netutil.UnixSockaddrToSockaddrSlice(unixsa, sa)
	assert.Nil(t, err)

	unixsa6, err := getUnixSockaddr("tcp6", "[2001:4860:0:2001::68]:54321")
	assert.Nil(t, err)
	err = netutil.UnixSockaddrToSockaddrSlice(unixsa6, sa)
	assert.Nil(t, err)

	// wrong sockaddrsize
	sa = make([]byte, netutil.SockaddrSize-1)
	err = netutil.UnixSockaddrToSockaddrSlice(unixsa, sa)
	assert.NotNil(t, err)

	err = netutil.UnixSockaddrToSockaddrSlice(unixsa6, sa)
	assert.NotNil(t, err)

}
