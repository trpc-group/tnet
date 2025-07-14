//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 Tencent.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

//go:build freebsd || dragonfly || darwin
// +build freebsd dragonfly darwin

package tnet

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/mcache"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

// FillToBuffer reads packets from UDP connection, and fills to buffer.
// If OS doesn't support UDP batch I/O, only one packet is received at a time.
func (nfd *netFD) FillToBuffer(b *buffer.Buffer) error {
	udpBufferSize, err := nfd.getUdpBufferSize()
	if err != nil {
		return fmt.Errorf("get udp buffer size: %w", err)
	}
	block := mcache.Malloc(udpBufferSize + netutil.SockaddrSize)
	n, sa, err := unix.Recvfrom(nfd.fd, block[netutil.SockaddrSize:], 0)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil
		}
		return fmt.Errorf("failed to read UDP packet: %w", err)
	}
	if err := netutil.UnixSockaddrToSockaddrSlice(sa, block[:netutil.SockaddrSize]); err != nil {
		return err
	}
	b.Write(false, block[:netutil.SockaddrSize+n])
	return nil
}

// getUdpBufferSize returns the size of the UDP buffer.
func (nfd *netFD) getUdpBufferSize() (int, error) {
	// Check if the exact buffer size retrieval is enabled.
	if !nfd.exactUDPBufferSizeEnabled {
		return nfd.udpBufferSize, nil
	}

	// If exact buffer size retrieval is enabled, attempt to peek at the incoming data
	// without removing it from the queue to determine the actual size of the buffer needed.
	n, _, err := unix.Recvfrom(nfd.fd, make([]byte, nfd.udpBufferSize), unix.MSG_PEEK)
	if err != nil {
		return 0, fmt.Errorf("recv from: %w", err)
	}
	return n, nil
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
	// Skip the n (here n == 1) block to prevent the same data from being written multiple times.
	if err := b.SkipBlocks(n); err != nil {
		return err
	}
	b.Release()
	return nil
}
