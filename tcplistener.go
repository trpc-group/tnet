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

package tnet

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/metrics"
)

type tcpListener struct {
	nfd netFD
}

type netError struct {
	error
	isTimeout bool
}

// Timeout implements net.Error interface.
func (e netError) Timeout() bool {
	return e.isTimeout
}

// Temporary implements net.Error interface.
func (e netError) Temporary() bool {
	switch {
	case errors.Is(e.error, unix.EAGAIN), errors.Is(e.error, unix.ECONNRESET), errors.Is(e.error, unix.ECONNABORTED):
		return true
	default:
	}
	// e.error is probably of type syscall.Errno,
	// make assertion to filter out more temporary errors (such as syscall.EMFILE).
	// Related material: https://github.com/golang/go/issues/6163
	t, ok := e.error.(temporary)
	return ok && t.Temporary()
}

type temporary interface {
	Temporary() bool
}

// Accept implements tcp listener's accept method.
func (t *tcpListener) Accept() (net.Conn, error) {
	// TODO: how to support blocking mode
	return t.accept(nil)
}

func (t *tcpListener) accept(handle OnTCPOpened) (net.Conn, error) {
	fd, sa, err := netutil.Accept(t.FD())
	if err != nil {
		return nil, netError{error: err}
	}

	// Get the local address of the new connection.
	localSA, err := unix.Getsockname(fd)
	var localAddr net.Addr
	if err == nil {
		localAddr = netutil.SockaddrToTCPOrUnixAddr(localSA)
	} else {
		// Fallback to listener address if getsockname fails.
		localAddr = t.nfd.laddr
	}

	conn := &tcpconn{
		nfd: netFD{
			fd:      fd,
			fdtype:  fdTCP,
			network: t.nfd.network,
			laddr:   localAddr,
			raddr:   netutil.SockaddrToTCPOrUnixAddr(sa),
		},
		readTrigger: make(chan struct{}, 1),
	}
	if !MassiveConnections.Load() {
		conn.writevData = iovec.NewIOData(iovec.WithLength(systype.MaxLen))
	}
	conn.inBuffer.Initialize()
	conn.outBuffer.Initialize()
	conn.closedReadBuf.Initialize(nil)
	if handle != nil {
		if err := handle(conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("on tcp opened error: %w", err)
		}
	}
	if err := conn.nfd.SetNoDelay(true); err != nil {
		return nil, fmt.Errorf("set tcp no delay error: %w", err)
	}
	if err := conn.nfd.Schedule(tcpOnRead, tcpOnWrite, tcpOnHup, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connection netfd schedule error: %w", err)
	}
	metrics.Add(metrics.TCPConnsCreate, 1)
	return conn, nil
}

// Close closes the tcp listener.
func (t *tcpListener) Close() error {
	t.nfd.close()
	return nil
}

// FD returns the tcp listener's file descriptor.
func (t *tcpListener) FD() (fd int) {
	return t.nfd.fd
}

// Addr returns the tcp listener's local address.
func (t *tcpListener) Addr() net.Addr {
	return t.nfd.laddr
}

func listenTCP(network string, address string) (*tcpListener, error) {
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return newListener(ln)
}

func newListener(listener net.Listener) (*tcpListener, error) {
	fd, err := netutil.GetFD(listener)
	if err != nil {
		return nil, fmt.Errorf("new listener get fd error: %w", err)
	}
	ln := &tcpListener{
		nfd: netFD{
			fd:      fd,
			fdtype:  fdListen,
			sock:    listener,
			network: listener.Addr().Network(),
			laddr:   listener.Addr(),
		},
	}
	return ln, nil
}
