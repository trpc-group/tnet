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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"go.uber.org/atomic"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/internal/stat"
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

var _ Restartable = (*tcpservice)(nil)

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
	s.connCond = sync.NewCond(&s.mu)
	return s, nil
}

const (
	initialTempDelay    = 5 * time.Millisecond
	maxTempDelay        = time.Second
	tempDelayMultiplier = 2
)

type tcpservice struct {
	ln         *tcpListener
	reqHandle  TCPHandler
	hupCh      chan struct{}
	conns      map[int]*tcpconn
	opts       options
	closed     atomic.Bool
	restarting atomic.Bool
	tempDelay  time.Duration
	mu         sync.Mutex
	hupOnce    sync.Once
	connCond   *sync.Cond
}

// Serve starts the service.
func (s *tcpservice) Serve(ctx context.Context) error {
	stat.Report(stat.ServerAttr, stat.TCPAttr)

	if err := s.ln.nfd.Schedule(tcpServiceOnRead, nil, tcpServiceOnHup, s); err != nil {
		return err
	}

	log.Infof("tnet tcp service started, current number of pollers: %d, use tnet.SetNumPollers to change it\n",
		poller.NumPollers())

	select {
	case <-ctx.Done():
		_ = s.close()
		return ctx.Err()
	case <-s.hupCh:
		if s.restarting.Load() {
			return s.waitConnections(ctx)
		}
		_ = s.close()
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
		if err := tconn.SetWriteIdleTimeout(s.opts.tcpWriteIdleTimeout); err != nil {
			return fmt.Errorf("tnet connection set write idle timeout error: %w", err)
		}
		if err := tconn.SetReadIdleTimeout(s.opts.tcpReadIdleTimeout); err != nil {
			return fmt.Errorf("tnet connection set read idle timeout error: %w", err)
		}
		tconn.outboundBufferLimit = s.opts.tcpOutboundBufferLimit
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
		var ne net.Error
		if errors.As(err, &ne) && ne.Temporary() {
			// Do not spin on temporary accept failure.
			// Reference:
			//   https://github.com/golang/go/commit/913abfee3bd25af5d80b3b9079d22f8e296d94c8
			s.doTempDelay()
			return nil
		}
		return fmt.Errorf("tcp service on read error during accepting: %w", err)
	}
	// Reset temporary delay so long as `accept` successfully returns.
	s.tempDelay = 0
	return nil
}

func (s *tcpservice) doTempDelay() {
	if s.tempDelay == 0 {
		s.tempDelay = initialTempDelay
	} else {
		s.tempDelay *= tempDelayMultiplier
	}
	if s.tempDelay > maxTempDelay {
		s.tempDelay = maxTempDelay
	}
	// The poller the current listener is in only handles listener events,
	// so sleep here may affect the `accept` events of other listeners (if there are more than one)
	// but not the connection's own events (since they will not be in the same poller as the listener).
	time.Sleep(s.tempDelay)
}

func tcpServiceOnHup(data interface{}) {
	s, ok := data.(*tcpservice)
	if !ok || s == nil {
		panic(fmt.Sprintf("bug: data is not *tcpservice type (%v) or s is nil pointer (%v)", !ok, s == nil))
	}
	s.hupOnce.Do(func() {
		close(s.hupCh)
	})
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
	s.mu.Lock()
	if _, ok := s.conns[conn.nfd.FD()]; ok {
		delete(s.conns, conn.nfd.FD())
		s.connCond.Broadcast()
	}
	s.mu.Unlock()
}

func (s *tcpservice) closeAll() {
	if !s.closed.Load() {
		return
	}
	s.mu.Lock()
	conns := make([]*tcpconn, 0, len(s.conns))
	for k, conn := range s.conns {
		conns = append(conns, conn)
		delete(s.conns, k)
	}
	s.connCond.Broadcast()
	s.mu.Unlock()
	for _, conn := range conns {
		conn.Close()
	}
}

func (s *tcpservice) waitConnections(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.connCond.Broadcast()
			s.mu.Unlock()
		case <-stop:
		}
	}()
	defer close(stop)

	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.conns) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.connCond.Wait()
	}
	return ctx.Err()
}

// Restart starts a new process, closes the listener, and waits for active TCP connections to drain.
func (s *tcpservice) Restart(ctx context.Context) error {
	if s.closed.Load() {
		return errors.New("service is closed")
	}
	if !s.restarting.CAS(false, true) {
		return errors.New("service is already restarting")
	}

	file := os.NewFile(uintptr(s.ln.FD()), gracefulListenerFileName)
	cmd := execCommand(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanAndAppendEnv(os.Environ(), gracefulRestartFDEnv, gracefulRestartFD)
	cmd.ExtraFiles = []*os.File{file}
	if err := cmd.Start(); err != nil {
		s.restarting.Store(false)
		return err
	}

	time.Sleep(s.opts.gracefulRestartTimeout)
	if err := s.ln.Close(); err != nil {
		return err
	}
	tcpServiceOnHup(s)
	return s.waitConnections(ctx)
}
