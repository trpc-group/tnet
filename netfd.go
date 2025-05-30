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
	"errors"
	"fmt"
	"net"
	"sync"
	"unsafe"

	"go.uber.org/atomic"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/metrics"
)

// goSockCloser is used to store go net library conn and listener.
type goSockCloser interface {
	Close() error
}

type fdType int

const (
	fdTCP fdType = iota
	fdUDP
	fdListen
)

type netFD struct {
	desc    *poller.Desc
	sock    goSockCloser
	laddr   net.Addr
	raddr   net.Addr
	network string

	fd     int
	fdtype fdType
	closed atomic.Bool

	// The intention of locker is to ensure close() concurrent safe.
	// netFD can only be closed once, and no control() can be called thereafter.
	locker                    sync.Mutex
	udpBufferSize             int
	exactUDPBufferSizeEnabled bool
}

var listenerPollMgr *poller.PollMgr

func init() {
	var err error
	listenerPollMgr, err = poller.NewPollMgr(
		poller.RoundRobin, 1,
		poller.WithIgnoreTaskError(true), // Ignore accept errors to prevent close of the listener.
	)
	if err != nil {
		panic("can't create listener pollmgr")
	}
}

// FD returns the netFD's file descriptor.
func (nfd *netFD) FD() int {
	return nfd.fd
}

// LocalAddr returns the local network address.
func (nfd *netFD) LocalAddr() net.Addr {
	return nfd.laddr
}

// RemoteAddr returns the remote network address.
func (nfd *netFD) RemoteAddr() net.Addr {
	return nfd.raddr
}

// SetKeepAlive sets the keep alive behavior of this net fd.
func (nfd *netFD) SetKeepAlive(secs int) error {
	return netutil.SetKeepAlive(nfd.fd, secs)
}

// SetNoDelay sets the TCP_NODELAY flag on this net fd.
func (nfd *netFD) SetNoDelay(noDelay bool) error {
	var v int
	if noDelay {
		v = 1
	}
	return unix.SetsockoptInt(nfd.fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, v)
}

// close is safe for concurrent call.
func (nfd *netFD) close() {
	nfd.locker.Lock()
	defer nfd.locker.Unlock()
	if !nfd.closed.CAS(false, true) {
		return
	}
	if nfd.desc != nil {
		nfd.desc.Close()
		poller.FreeDesc(nfd.desc)
		nfd.desc = nil
	}
	if nfd.sock != nil {
		nfd.sock.Close()
	} else {
		unix.Close(nfd.fd)
	}
}

// Schedule add NetFD to poller system, and monitor Readable Event.
func (nfd *netFD) Schedule(
	onRead func(data interface{}, ioData *iovec.IOData) error,
	onWrite func(data interface{}) error,
	onHup func(data interface{}),
	conn interface{},
) error {
	if nfd.desc != nil {
		return errors.New("already in poller system")
	}
	desc := poller.NewDesc()
	desc.Lock()
	desc.FD = nfd.FD()
	desc.Data = conn
	desc.OnRead, desc.OnWrite, desc.OnHup = onRead, onWrite, onHup
	desc.Unlock()
	var err error
	if nfd.fdtype == fdListen {
		err = desc.PickPollerWithPollMgr(listenerPollMgr)
	} else {
		err = desc.PickPoller()
	}
	if err != nil {
		poller.FreeDesc(desc)
		return err
	}
	nfd.locker.Lock()
	nfd.desc = desc
	nfd.locker.Unlock()
	return nfd.Control(poller.Readable)
}

// Control register interest event to poller system.
func (nfd *netFD) Control(event poller.Event) error {
	nfd.locker.Lock()
	defer nfd.locker.Unlock()
	if nfd.closed.Load() {
		return ErrConnClosed
	}
	if nfd.desc == nil {
		return fmt.Errorf("netFD %d is not add to poller", nfd.FD())
	}
	return nfd.desc.Control(event)
}

// Readv implements batch receive packets from socket.
func (nfd *netFD) Readv(ivs []unix.Iovec) (int, error) {
	if len(ivs) == 0 {
		return 0, nil
	}
	r, _, e := unix.RawSyscall(unix.SYS_READV, uintptr(nfd.fd), uintptr(unsafe.Pointer(&ivs[0])), uintptr(len(ivs)))
	metrics.Add(metrics.TCPReadvCalls, 1)
	if e != 0 {
		metrics.Add(metrics.TCPReadvFails, 1)
		return int(r), unix.Errno(e)
	}
	metrics.Add(metrics.TCPReadvBytes, uint64(r))
	return int(r), nil
}

// Writev implements batch send packets to socket.
func (nfd *netFD) Writev(ivs []unix.Iovec) (int, error) {
	if len(ivs) == 0 {
		return 0, nil
	}
	r, _, e := unix.RawSyscall(unix.SYS_WRITEV, uintptr(nfd.fd), uintptr(unsafe.Pointer(&ivs[0])), uintptr(len(ivs)))
	metrics.Add(metrics.TCPWritevCalls, 1)
	if e != 0 {
		metrics.Add(metrics.TCPWritevFails, 1)
		return int(r), unix.Errno(e)
	}
	metrics.Add(metrics.TCPWritevBlocks, uint64(len(ivs)))
	return int(r), nil
}

const (
	defaultUDPBufferSize             = 65535
	defaultExactUDPBufferSizeEnabled = false
)

var (
	udpPacketNum = 32
)

// WriteTo writes a packet with payload p to addr.
func (nfd *netFD) WriteTo(data []byte, addr net.Addr) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if addr == nil {
		return 0, errors.New("address can't be nil")
	}
	if len(data) > nfd.udpBufferSize {
		return 0, fmt.Errorf("data length %d is too long, the max udp buffer size is %d", len(data), nfd.udpBufferSize)
	}
	sa, err := netutil.AddrToSockAddr(nfd.laddr, addr)
	if err != nil {
		return 0, err
	}
	return len(data), unix.Sendto(nfd.FD(), data, 0, sa)
}
