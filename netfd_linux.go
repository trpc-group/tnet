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

//go:build linux
// +build linux

package tnet

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/mcache"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/metrics"
)

// SendPackets sends UDP packets from buffer.
func (nfd *netFD) SendPackets(b *buffer.Buffer) error {
	return nfd.sendMMsg(b)
}

// FillToBuffer reads packets from UDP connection, and fills the buffer.
func (nfd *netFD) FillToBuffer(b *buffer.Buffer) error {
	if nfd.exactUDPBufferSizeEnabled {
		return nfd.fillToExactBuffer(b)
	}
	// Prepare mmsgs used to SYS_RECVMMSG syscall.
	mmsgs := systype.GetMMsghdrs(udpPacketNum)
	defer systype.PutMMsghdrs(mmsgs)
	bufs, w := systype.GetIOData(udpPacketNum)
	if w != nil {
		defer systype.PutIOData(w)
	}
	n := nfd.udpBufferSize + netutil.SockaddrSize
	for i := 0; i < udpPacketNum; i++ {
		bufs[i] = mcache.Malloc(n)
	}
	buildMMsgs(mmsgs, bufs)

	// Call SYS_RECVMMSG to receive data from fd.
	r, err := nfd.syscallMMsg(unix.SYS_RECVMMSG, mmsgs)
	metrics.Add(metrics.UDPRecvMMsgCalls, 1)
	if err != nil {
		metrics.Add(metrics.UDPRecvMMsgFails, 1)
		return err
	}
	metrics.Add(metrics.UDPRecvMMsgPackets, uint64(r))

	// The actual received data may be less than the pre-allocated
	// space, adjust the length of the bufs to the actual received
	// data, and then write to the buffer.
	for i := 0; i < r; i++ {
		l := mmsgs[i].Len
		bufs[i] = bufs[i][:netutil.SockaddrSize+l]
	}
	for i := r; i < udpPacketNum; i++ {
		mcache.Free(bufs[i])
	}
	bufs = bufs[:r]
	b.Writev(false, bufs...)
	return nil
}

// fillToExactBuffer reads packets from UDP connection with exact buffer size, and fills the buffer.
func (nfd *netFD) fillToExactBuffer(b *buffer.Buffer) error {
	// Prepare msg used to SYS_RECVMSG syscall.
	msg := systype.GetMsghdr()
	defer systype.PutMsghdr(msg)

	// Call SYS_RECVMSG with MSG_PEEK | MSG_TRUNC flags to obtain the size of the UDP packet.
	udpBufferSize, err := nfd.syscallMsg(unix.SYS_RECVMSG, msg, unix.MSG_PEEK|unix.MSG_TRUNC)
	metrics.Add(metrics.UDPRecvMsgCalls, 1)
	if err != nil {
		metrics.Add(metrics.UDPRecvMsgFails, 1)
		return err
	}

	// Allocate a buffer of the exact size needed for the UDP packet.
	buf := mcache.Malloc(udpBufferSize + netutil.SockaddrSize)
	buildMsg(msg, buf)

	// Call SYS_RECVMSG to read the UDP packet.
	_, err = nfd.syscallMsg(unix.SYS_RECVMSG, msg, 0)
	if err != nil {
		metrics.Add(metrics.UDPRecvMsgFails, 1)
		return err
	}
	metrics.Add(metrics.UDPRecvMsgPackets, 1)

	// Write the received data into the buffer.
	buf = buf[:netutil.SockaddrSize+udpBufferSize]
	b.Writev(false, buf)
	return nil
}

// SendMMsg batch sends UDP packets from buffer.
func (nfd *netFD) sendMMsg(b *buffer.Buffer) error {
	mmsgs := systype.GetMMsghdrs(udpPacketNum)
	defer systype.PutMMsghdrs(mmsgs)
	bufs, w := systype.GetIOData(udpPacketNum)
	if w != nil {
		defer systype.PutIOData(w)
	}

	l := b.PeekBlocks(bufs)
	mmsgs, bufs = mmsgs[:l], bufs[:l]

	buildMMsgs(mmsgs, bufs)

	n, err := nfd.syscallMMsg(unix.SYS_SENDMMSG, mmsgs)
	metrics.Add(metrics.UDPSendMMsgCalls, 1)
	if err != nil {
		metrics.Add(metrics.UDPSendMMsgFails, 1)
		return err
	}
	metrics.Add(metrics.UDPSendMMsgPackets, uint64(n))
	if err := b.SkipBlocks(n); err != nil {
		return err
	}
	b.Release()
	return err
}

func buildMMsgs(mmsgs []systype.MMsghdr, bufs [][]byte) error {
	if len(mmsgs) != len(bufs) {
		return errors.New("buffers length is not equal to MMsghdrs length")
	}
	for i := range mmsgs {
		if len(bufs[i]) < netutil.SockaddrSize {
			return fmt.Errorf("invalid buffer size: buffer length is %d, required minimum is %d",
				len(bufs[i]), netutil.SockaddrSize)
		}
		buf := bufs[i][netutil.SockaddrSize:]
		name := bufs[i][:netutil.SockaddrSize]
		systype.BuildMMsg(&mmsgs[i], name, buf)
	}
	return nil
}

func buildMsg(msg *systype.Msghdr, buf []byte) error {
	if len(buf) < netutil.SockaddrSize {
		return fmt.Errorf("invalid buffer size: buffer length is %d, required minimum is %d",
			len(buf), netutil.SockaddrSize)
	}
	name := buf[:netutil.SockaddrSize]
	buf = buf[netutil.SockaddrSize:]
	systype.BuildMsg(msg, name, buf)
	return nil
}

func (nfd *netFD) syscallMMsg(trap int, mmsgs []systype.MMsghdr) (int, error) {
	switch trap {
	case unix.SYS_SENDMMSG:
	case unix.SYS_RECVMMSG:
	default:
		return 0, fmt.Errorf("unsupported MMsg syscall type: %d. Supported types: SYS_SENDMMSG(%d), SYS_RECVMMSG(%d)",
			trap, unix.SYS_SENDMMSG, unix.SYS_RECVMMSG)
	}
	r, _, e := unix.Syscall6(
		uintptr(trap),
		uintptr(nfd.fd),
		uintptr(unsafe.Pointer(&mmsgs[0])),
		uintptr(len(mmsgs)),
		0, 0, 0)
	if e != 0 {
		return int(r), unix.Errno(e)
	}
	return int(r), nil
}

func (nfd *netFD) syscallMsg(trap int, msg *systype.Msghdr, flag int) (int, error) {
	switch trap {
	case unix.SYS_RECVMSG:
	default:
		return 0, fmt.Errorf("unsupported Msg syscall type: %d. Supported types: SYS_RECVMSG(%d)",
			trap, unix.SYS_RECVMSG)
	}
	r, _, e := unix.Syscall6(
		uintptr(trap),
		uintptr(nfd.fd),
		uintptr(unsafe.Pointer(msg)),
		uintptr(flag),
		0, 0, 0)
	if e != 0 {
		return int(r), unix.Errno(e)
	}
	return int(r), nil
}
