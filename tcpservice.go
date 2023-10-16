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

	"go.uber.org/atomic"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/log"
)

// NewTCPService creates a tcp Service and binds it to a listener. It is recommended to
// create listener by func tnet.Listen, otherwise make sure that listener implements
// syscall.Conn interface.
//
//	type syscall.Conn interface {
//		SyscallConn() (RawConn, error)
//	}
func NewTCPService(listener net.Listener, handler TCPHandler, opt ...Option) (Service, error) {
	if listener == nil {
		return nil, errors.New("listener is nil")
	}
	ln, ok := listener.(*tcpListener)
	if ok {
		return newTCPService(ln, handler, opt...)
	}

	if err := netutil.ValidateTCP(listener); err != nil {
		return nil, fmt.Errorf("validate listener fail: %w", err)
	}
	// Not of our customized type? Wrap one!
	ln, err := newListener(listener)
	if err != nil {
		return nil, err
	}
	return newTCPService(ln, handler, opt...)
}

func newTCPService(ln *tcpListener, handler TCPHandler, opt ...Option) (Service, error) {
	opts := options{}
	opts.setDefault()
	for _, o := range opt {
		o.f(&opts)
	}

	s := &tcpservice{
		ln:        ln,
		reqHandle: handler,
		opts:      opts,
		conns:     make(map[int]*tcpconn),
		hupCh:     make(chan struct{}),
	}
	return s, nil
}

type tcpservice struct {
	ln        *tcpListener
	reqHandle TCPHandler
	hupCh     chan struct{}
	conns     map[int]*tcpconn
	opts      options
	closed    atomic.Bool
	mu        sync.Mutex
}

// Serve starts the service.
func (s *tcpservice) Serve(ctx context.Context) error {
	if err := s.ln.nfd.Schedule(tcpServiceOnRead, nil, tcpServiceOnHup, s); err != nil {
		return err
	}

	log.Infof("tnet tcp service started, current number of pollers: %d, use tnet.SetNumPollers to change it\n",
		poller.NumPollers())

	defer s.close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.hupCh:
		return errors.New("listener is closed")
	}
}

func (s *tcpservice) close() error {
	if s.ln == nil {
		return nil
	}
	s.closed.Store(true)
	s.closeAll()
	return s.ln.Close()
}

// tcpServiceOnRead is triggered by the tcp listener read event,
// which means that "accept" needs to be handled.
func tcpServiceOnRead(data interface{}, _ *iovec.IOData) error {
	s, ok := data.(*tcpservice)
	if !ok || s == nil {
		panic(fmt.Sprintf("bug: data is not *tcpservice type (%v) or s is nil pointer (%v)", !ok, s == nil))
	}
	if s.closed.Load() {
		return errors.New("service is closed")
	}
	openHandle := func(conn Conn) error {
		tconn, ok := conn.(*tcpconn)
		if !ok {
			return errors.New("bug: conn is not tcpconn type")
		}
		if err := tconn.SetOnRequest(s.reqHandle); err != nil {
			return fmt.Errorf("tnet connection set on request error: %w", err)
		}
		if err := tconn.SetKeepAlive(s.opts.tcpKeepAlive); err != nil {
			return fmt.Errorf("tnet connection set keep alive error: %w", err)
		}
		if err := tconn.SetIdleTimeout(s.opts.tcpIdleTimeout); err != nil {
			return fmt.Errorf("tnet connection set idle timeout error: %w", err)
		}
		tconn.SetNonBlocking(s.opts.nonblocking)
		tconn.SetSafeWrite(s.opts.safeWrite)
		if s.opts.onTCPClosed != nil {
			tconn.SetOnClosed(s.opts.onTCPClosed)
		}
		tconn.service = s
		s.storeConn(tconn)
		// Execute the hook function set by the user for tcp connection creation.
		if s.opts.onTCPOpened != nil {
			return s.opts.onTCPOpened(tconn)
		}
		return nil
	}
	if _, err := s.ln.accept(openHandle); err != nil {
		if ne, ok := err.(net.Error); ok && ne.Temporary() {
			return nil
		}
		return fmt.Errorf("tcp service on read error during accepting: %w", err)
	}
	return nil
}

func tcpServiceOnHup(data interface{}) {
	s, ok := data.(*tcpservice)
	if !ok || s == nil {
		panic(fmt.Sprintf("bug: data is not *tcpservice type (%v) or s is nil pointer (%v)", !ok, s == nil))
	}
	close(s.hupCh)
}

func (s *tcpservice) storeConn(conn *tcpconn) {
	if s.closed.Load() {
		return
	}
	s.mu.Lock()
	s.conns[conn.nfd.FD()] = conn
	s.mu.Unlock()
}

func (s *tcpservice) deleteConn(conn *tcpconn) {
	if s.closed.Load() {
		return
	}
	s.mu.Lock()
	delete(s.conns, conn.nfd.FD())
	s.mu.Unlock()
}

func (s *tcpservice) closeAll() {
	if !s.closed.Load() {
		return
	}
	s.mu.Lock()
	for k, conn := range s.conns {
		conn.Close()
		delete(s.conns, k)
	}
	s.mu.Unlock()
}
