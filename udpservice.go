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
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	goreuseport "github.com/kavu/go_reuseport"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/internal/stat"
	"trpc.group/trpc-go/tnet/log"
)

// NewUDPService creates a udp service. Ensure that all listeners are listening to the same address.
func NewUDPService(lns []PacketConn, handler UDPHandler, opt ...Option) (Service, error) {
	if err := validateListeners(lns); err != nil {
		return nil, err
	}
	return newUDPService(lns, handler, opt...)
}

func newUDPService(lns []PacketConn, handler UDPHandler, opt ...Option) (Service, error) {
	var opts options
	opts.setDefault()
	for _, o := range opt {
		o.f(&opts)
	}
	wg := &sync.WaitGroup{}
	wg.Add(len(lns))
	s := &udpservice{
		reqHandle:     handler,
		opts:          opts,
		allConnClosed: wg,
	}
	for _, ln := range lns {
		conn, ok := ln.(*udpconn)
		if !ok {
			return nil, fmt.Errorf("listeners are not of udpconn type: %T, they should be created by tnet.ListenPackets", ln)
		}
		conn.SetMaxPacketSize(s.opts.maxUDPPacketSize)
		conn.SetExactUDPBufferSizeEnabled(s.opts.exactUDPBufferSizeEnabled)
		conn.closeService = wg
		s.conns = append(s.conns, conn)
	}
	return s, nil
}

// NewPacketConn creates a tnet.PacketConn from net.PacketConn. Note that
// conn must listen on UDP and make sure that conn implements syscall.Conn.
func NewPacketConn(conn net.PacketConn) (PacketConn, error) {
	if err := netutil.ValidateUDP(conn); err != nil {
		return nil, fmt.Errorf("validate listener fail: %w", err)
	}
	uc, err := newUDPConn(conn)
	if err != nil {
		return nil, err
	}
	return uc, nil
}

func listenUDP(network string, address string, reuseport bool) ([]PacketConn, error) {
	var lns []PacketConn
	n := 1
	listenPacket := net.ListenPacket
	if reuseport {
		n = poller.NumPollers()
		listenPacket = goreuseport.ListenPacket
	}
	for i := 0; i < n; i++ {
		rawConn, err := listenPacket(network, address)
		if err != nil {
			return nil, fmt.Errorf("udp listen error:%v", err)
		}
		conn, err := newUDPConn(rawConn)
		if err != nil {
			return nil, err
		}
		lns = append(lns, conn)
		// Set the address with a specified port to prevent the user from listening on a random port.
		address = rawConn.LocalAddr().String()
	}
	return lns, nil
}

func newUDPConn(listener net.PacketConn) (*udpconn, error) {
	fd, err := netutil.GetFD(listener)
	if err != nil {
		listener.Close()
		return nil, err
	}
	conn := &udpconn{
		nfd: netFD{
			fd:            fd,
			fdtype:        fdUDP,
			sock:          listener,
			network:       listener.LocalAddr().Network(),
			laddr:         listener.LocalAddr(),
			udpBufferSize: defaultUDPBufferSize,
		},
		readTrigger: make(chan struct{}, 1),
	}
	conn.inBuffer.Initialize()
	conn.outBuffer.Initialize()
	return conn, nil
}

type udpservice struct {
	reqHandle     UDPHandler
	conns         []*udpconn
	opts          options
	allConnClosed *sync.WaitGroup
}

// Serve starts the service.
func (s *udpservice) Serve(ctx context.Context) error {
	stat.Report(stat.ServerAttr, stat.UDPAttr)

	defer s.close()
	for _, conn := range s.conns {
		if err := conn.SetOnRequest(s.reqHandle); err != nil {
			return err
		}
		conn.SetNonBlocking(s.opts.nonblocking)

		if s.opts.onUDPClosed != nil {
			conn.SetOnClosed(s.opts.onUDPClosed)
		}
		if err := conn.schedule(); err != nil {
			return err
		}
	}

	log.Infof("tnet udp service started, current number of pollers: %d, use tnet.SetNumPollers to change it\n",
		poller.NumPollers())

	allConnClosed := make(chan struct{})
	go func() {
		s.allConnClosed.Wait()
		close(allConnClosed)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-allConnClosed:
		return errors.New("all connection is closed")
	}
}

func (s *udpservice) close() error {
	for _, conn := range s.conns {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

func validateListeners(lns []PacketConn) error {
	if len(lns) == 0 {
		return errors.New("listeners can't be nil")
	}
	// Ensure that all listeners are listening to the same address.
	firstAddr := lns[0].LocalAddr()
	for i := 1; i < len(lns); i++ {
		if addr := lns[i].LocalAddr(); addr.String() != firstAddr.String() {
			return fmt.Errorf("listeners have different local address: %s, %s", firstAddr, addr)
		}
	}
	return nil
}
