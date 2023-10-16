// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build freebsd || dragonfly || darwin
// +build freebsd dragonfly darwin

package tnet

import (
	"errors"

	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/mcache"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

// FillToBuffer reads packets from UDP connection, and fills to buffer.
// If OS doesn't support UDP batch I/O, only one packet is received at a time.
func (nfd *netFD) FillToBuffer(b *buffer.Buffer) error {
	block := mcache.Malloc(nfd.udpBufferSize + netutil.SockaddrSize)
	n, sa, err := unix.Recvfrom(nfd.fd, block[netutil.SockaddrSize:], 0)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil
		}
		return errors.New("failed to read UDP packet")
	}
	netutil.UnixSockaddrToSockaddrSlice(sa, block[:netutil.SockaddrSize])
	if err != nil {
		return err
	}
	b.Write(false, block[:netutil.SockaddrSize+n])
	return nil
}

// SendPackets sends UDP packets from buffer.
// If OS doesn't support UDP batch I/O, only one packet is sent at a time
func (nfd *netFD) SendPackets(b *buffer.Buffer) error {
	block := make([][]byte, 1)
	n := b.PeekBlocks(block)
	if n != 1 {
		return errors.New("block numbers is unexpected")
	}
	buf, addr, err := getUDPDataAndAddr(block[0])
	if err != nil {
		return err
	}
	nfd.WriteTo(buf, addr)
	return nil
}
