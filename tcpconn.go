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
	"fmt"
	"math"
	"net"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/asynctimer"
	"trpc.group/trpc-go/tnet/internal/autopostpone"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/locker"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/internal/timer"
	"trpc.group/trpc-go/tnet/log"
	"trpc.group/trpc-go/tnet/metrics"
)

const (
	// defaultTCPKeepAlive is a default constant value for TCPKeepAlive times.
	defaultTCPKeepAlive = 15 * time.Second
	// defaultCleanUpCheckInterval is interval time to check whether connections
	// number is greater than defaultCleanUpThrottle and enable clean up feature.
	defaultCleanUpCheckInterval = time.Second
)

var (
	// DefaultCleanUpThrottle is a default connections number throttle to determine
	// whether to enable buffer clean up feature.
	DefaultCleanUpThrottle = 10000
	// ErrConnClosed connection is closed.
	ErrConnClosed = netError{error: errors.New("conn is closed")}
	// EAGAIN represents error of not enough data.
	EAGAIN = netError{error: errors.New("no enough data, try it again")}
)

// tcpconn must implements Conn interface.
var _ Conn = (*tcpconn)(nil)

type tcpconn struct {
	service     *tcpservice
	metaData    any
	reqHandle   atomic.Value
	closeHandle atomic.Value
	readTrigger chan struct{}
	inBuffer    buffer.Buffer
	outBuffer   buffer.Buffer
	rtimer      *timer.Timer
	wtimer      *timer.Timer
	idleTimer   *asynctimer.Timer
	writevData  iovec.IOData
	nfd         netFD

	closer
	postpone    autopostpone.PostponeWrite
	waitReadLen atomic.Int32
	reading     locker.Locker
	writing     locker.Locker
	nonblocking bool
	safeWrite   bool
}

// MassiveConnections denotes whether this is under heavy connections scenario.
var MassiveConnections bool

func init() {
	go checkAndSetBufferCleanUp()
}

func checkAndSetBufferCleanUp() {
	ticker := time.NewTicker(defaultCleanUpCheckInterval)
	for range ticker.C {
		if metrics.Get(metrics.TCPConnsCreate)-
			metrics.Get(metrics.TCPConnsClose) > uint64(DefaultCleanUpThrottle) {
			buffer.SetCleanUp(true)
			MassiveConnections = true
		} else {
			buffer.SetCleanUp(false)
			MassiveConnections = false
		}
	}
}

// Read reads data from the tcpconn.
func (tc *tcpconn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	if !tc.beginJobSafely(apiRead) {
		return 0, ErrConnClosed
	}
	defer tc.endJobSafely(apiRead)

	if err := tc.waitRead(1); err != nil {
		return 0, err
	}
	return tc.inBuffer.Read(b)
}

// ReadN reads fixed length of data from the tcpconn.
func (tc *tcpconn) ReadN(n int) ([]byte, error) {
	if !tc.beginJobSafely(apiRead) {
		return nil, ErrConnClosed
	}
	defer tc.endJobSafely(apiRead)

	if err := tc.waitRead(n); err != nil {
		return nil, err
	}
	dst := make([]byte, n)
	_, err := tc.inBuffer.Read(dst)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

// Next reads fixed length of data from the tcpconn.
func (tc *tcpconn) Next(n int) ([]byte, error) {
	if !tc.beginJobSafely(apiRead) {
		return nil, ErrConnClosed
	}
	defer tc.endJobSafely(apiRead)

	if err := tc.waitRead(n); err != nil {
		return nil, err
	}
	return tc.inBuffer.Next(n)
}

// Peek returns the next n bytes without advancing the reader. It waits until it has
// read at least n bytes or error has occurred such as connection closed or read timeout.
// The bytes stop being valid at the next ReadN or Release call.
func (tc *tcpconn) Peek(n int) ([]byte, error) {
	if !tc.beginJobSafely(apiRead) {
		return nil, ErrConnClosed
	}
	defer tc.endJobSafely(apiRead)

	if err := tc.waitRead(n); err != nil {
		return nil, err
	}
	return tc.inBuffer.Peek(n)
}

// Skip skips the next n bytes and advances the reader. It waits until the underlayer has at
// least n bytes or error has occurred such as connection closed or read timeout.
func (tc *tcpconn) Skip(n int) error {
	if !tc.beginJobSafely(apiRead) {
		return ErrConnClosed
	}
	defer tc.endJobSafely(apiRead)

	if err := tc.waitRead(n); err != nil {
		return err
	}
	return tc.inBuffer.Skip(n)
}

// Release releases underlayer buffer when using Peek() and Skip() Zero-Copy APIs.
func (tc *tcpconn) Release() {
	if !tc.beginJobSafely(apiRead) {
		return
	}
	defer tc.endJobSafely(apiRead)
	tc.inBuffer.Release()
}

func (tc *tcpconn) waitRead(n int) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if tc.inBuffer.LenRead() >= n {
		return nil
	}

	tc.waitReadLen.Store(int32(n))
	if tc.nonblocking {
		return EAGAIN
	}

	defer tc.waitReadLen.Store(0)
	if tc.rtimer != nil && !tc.rtimer.IsZero() {
		return tc.waitReadWithTimeout(n)
	}

	for tc.inBuffer.LenRead() < n {
		if !tc.IsActive() {
			return ErrConnClosed
		}
		<-tc.readTrigger
	}
	return nil
}

func (tc *tcpconn) timeoutError() error {
	err := fmt.Errorf("read tcp %s->%s: i/o timeout",
		tc.LocalAddr().String(), tc.RemoteAddr().String())
	return netError{error: err, isTimeout: true}
}

func (tc *tcpconn) waitReadWithTimeout(n int) error {
	tc.rtimer.Start()
	select {
	case <-tc.rtimer.Wait():
		return tc.timeoutError()
	default:
	}
	for tc.inBuffer.LenRead() < n {
		if !tc.IsActive() {
			return ErrConnClosed
		}
		select {
		case <-tc.readTrigger:
			continue
		case <-tc.rtimer.Wait():
			return tc.timeoutError()
		}
	}
	return nil
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (tc *tcpconn) Write(b []byte) (int, error) {
	return tc.Writev(b)
}

// Writev provides multiple data slice write in order.
func (tc *tcpconn) Writev(p ...[]byte) (int, error) {
	if tc.wtimer != nil && tc.wtimer.Expired() {
		return 0, tc.timeoutError()
	}
	if !tc.beginJobSafely(apiWrite) {
		return 0, ErrConnClosed
	}
	n := tc.outBuffer.Writev(tc.safeWrite, p...)
	var err error
	if tc.postpone.Enabled() {
		err = tc.notify()
	} else {
		err = tc.flush()
	}
	if err != nil {
		tc.endJobSafely(apiWrite)
		tc.Close()
		return n, err
	}
	tc.endJobSafely(apiWrite)
	return n, nil
}

func (tc *tcpconn) writeToNetFD() error {
	tc.refreshConn()
	var (
		n   int
		err error
	)
	if tc.writevData.IsNil() {
		n, err = tc.writeWithCachedIOData()
	} else {
		n, err = tc.writeWithAdhocIOData()
	}
	if err != nil {
		return errors.Wrap(err, "tcpconn write with IOData")
	}
	if err := tc.outBuffer.Skip(n); err != nil {
		return errors.Wrap(err, fmt.Sprintf("tcpconn output buffer skip %d", n))
	}
	tc.outBuffer.Release()
	return nil
}

func (tc *tcpconn) writeWithCachedIOData() (int, error) {
	bs, w1 := systype.GetIOData(systype.MaxLen)
	if w1 != nil {
		defer systype.PutIOData(w1)
	}
	l := tc.outBuffer.PeekBlocks(bs)
	tc.postpone.CheckAndDisablePostponeWrite(l)
	ivs, w2 := systype.GetIOVECWrapper(bs[:l])
	if w2 != nil {
		defer systype.PutIOVECWrapper(w2)
	}
	n, err := tc.nfd.Writev(ivs)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (tc *tcpconn) writeWithAdhocIOData() (int, error) {
	l := tc.outBuffer.PeekBlocks(tc.writevData.ByteVec)
	tc.postpone.CheckAndDisablePostponeWrite(l)
	tc.writevData.SetIOVec(l)
	n, err := tc.nfd.Writev(tc.writevData.IOVec[:l])
	if err != nil {
		return 0, errors.Wrap(err, "tcpconn.writeToNetFD: nfd.Writev")
	}
	tc.writevData.Release(l)
	return n, nil
}

// notify asks poller to send data out.
func (tc *tcpconn) notify() error {
	if !tc.writing.TryLock() {
		return nil
	}
	metrics.Add(metrics.TCPWriteNotify, 1)
	return tc.nfd.Control(poller.ModReadWriteable)
}

// flush first try to write data directly, otherwise ask poller to write data
func (tc *tcpconn) flush() error {
	if !tc.writing.TryLock() {
		return nil
	}
	if err := tc.writeToNetFD(); err != nil {
		if !errors.Is(err, unix.EAGAIN) {
			return err
		}
		metrics.Add(metrics.TCPWriteNotify, 1)
		return tc.nfd.Control(poller.ModReadWriteable)
	}
	metrics.Add(metrics.TCPFlushCalls, 1)
	if tc.outBuffer.LenRead() != 0 {
		metrics.Add(metrics.TCPWriteNotify, 1)
		return tc.nfd.Control(poller.ModReadWriteable)
	}
	tc.writing.Unlock()

	if tc.outBuffer.LenRead() != 0 && tc.writing.TryLock() {
		metrics.Add(metrics.TCPWriteNotify, 1)
		return tc.nfd.Control(poller.ModReadWriteable)
	}
	return nil
}

// Close closes the tcpconn safely, it can be called multiple times concurrently.
func (tc *tcpconn) Close() error {
	if !tc.beginJobSafely(closeAll) {
		return nil
	}
	defer tc.endJobSafely(closeAll)
	// Stop OnRead event processing in poller.
	tc.closeJobSafely(sysRead)
	// Wakeup all read routines from blocking.
	close(tc.readTrigger)
	// Stop all jobs safely.
	tc.closeAllJobs()
	// Execute user-defined closing process.
	if closeHandle := tc.getOnClosed(); closeHandle != nil {
		closeHandle(tc)
	}
	// Stop all timers.
	if tc.rtimer != nil {
		tc.rtimer.Stop()
	}
	if tc.wtimer != nil {
		tc.wtimer.Stop()
	}
	// Delete conn from service conns map.
	if tc.service != nil {
		tc.service.deleteConn(tc)
	}
	if tc.idleTimer != nil {
		asynctimer.Del(tc.idleTimer)
	}
	// Safe to free netFD.
	tc.nfd.close()
	// Free input/output buffer.
	tc.inBuffer.Free()
	tc.outBuffer.Free()
	metrics.Add(metrics.TCPConnsClose, 1)
	return nil
}

// IsActive checks whether the tcpconn is active or not.
func (tc *tcpconn) IsActive() bool {
	return !tc.closed()
}

// LocalAddr returns the local network address.
func (tc *tcpconn) LocalAddr() net.Addr {
	return tc.nfd.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (tc *tcpconn) RemoteAddr() net.Addr {
	return tc.nfd.RemoteAddr()
}

// Len returns the total length of the readable data in the reader.
func (tc *tcpconn) Len() int {
	if !tc.beginJobSafely(apiCtrl) {
		return 0
	}
	defer tc.endJobSafely(apiCtrl)
	return tc.inBuffer.LenRead()
}

// SetOnClosed sets the additional close process for a connection.
// Handle is executed when the connection is closed.
func (tc *tcpconn) SetOnClosed(handle OnTCPClosed) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if handle == nil {
		return errors.New("onClosed can't be nil")
	}
	tc.closeHandle.Store(handle)
	return nil
}

// SetOnRequest can set or replace the TCPHandler method for a connection.
// Generally, on the server side the handler is set when the connection is established.
// On the client side, if necessary, make sure that TCPHandler is set before sending data.
func (tc *tcpconn) SetOnRequest(handle TCPHandler) error {
	if handle == nil {
		return errors.New("handle can't be nil")
	}
	tc.reqHandle.Store(handle)
	return nil
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
// A zero value for t means I/O operations will not time out.
func (tc *tcpconn) SetDeadline(t time.Time) error {
	if err := tc.SetReadDeadline(t); err != nil {
		return err
	}
	return tc.SetWriteDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// A zero value for t means Read will not time out.
func (tc *tcpconn) SetReadDeadline(t time.Time) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if tc.rtimer == nil {
		tc.rtimer = timer.New(t)
		return nil
	}
	tc.rtimer.Reset(t)
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// A zero value for t means Write will not time out.
func (tc *tcpconn) SetWriteDeadline(t time.Time) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if tc.wtimer == nil {
		tc.wtimer = timer.New(t)
		return nil
	}
	tc.wtimer.Reset(t)
	return nil
}

// SetKeepAlive sets keep alive time for tcp connection.
// By default, keep alive is turned on with value defaultKeepAlive.
// If keepAlive <= 0, keep alive will be turned off.
// Otherwise, keep alive value will be round up to seconds.
func (tc *tcpconn) SetKeepAlive(t time.Duration) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if t <= 0 {
		// Turn off keep alive.
		return nil
	}
	return tc.nfd.SetKeepAlive(int(math.Ceil(t.Seconds())))
}

// SetIdleTimeout sets the idle timeout to close connection.
func (tc *tcpconn) SetIdleTimeout(d time.Duration) error {
	if !tc.IsActive() {
		return ErrConnClosed
	}
	if d <= 0 {
		return nil
	}
	if tc.idleTimer != nil {
		asynctimer.Del(tc.idleTimer)
	}
	tc.idleTimer = asynctimer.NewTimer(tc, tcpOnIdle, d)
	if err := asynctimer.Add(tc.idleTimer); err != nil {
		return fmt.Errorf("tcp connection set idle timeout asynctimer add error: %w", err)
	}
	return nil
}

// SetNonBlocking sets conn to nonblocking. Read APIs will return EAGAIN when there is not
// enough data for reading.
func (tc *tcpconn) SetNonBlocking(nonblock bool) {
	tc.nonblocking = nonblock
}

// SetFlushWrite sets whether to flush the data or not.
// Default behavior is notify.
// Deprecated: whether enable this feature is controlled by system automatically.
func (tc *tcpconn) SetFlushWrite(flushWrite bool) {}

// SetSafeWrite sets whether write on connection is safe or not.
// Default is unsafe.
//
// This option affects the behavior of Write/Writev.
//
//	If safeWrite = false: the lifetime of buffers passed into Write/Writev will
//	  be handled by tnet, which means users cannot reuse the buffers after passing
//	  them into Write/Writev.
//	If safeWrite = true: the given buffers is copied into tnet's own buffer.
//	  Therefore users can reuse the buffers passed into Write/Writev.
func (tc *tcpconn) SetSafeWrite(safeWrite bool) {
	tc.safeWrite = safeWrite
}

func (tc *tcpconn) getOnRequest() TCPHandler {
	handler := tc.reqHandle.Load()
	if handler == nil {
		return nil
	}
	reqHandle, ok := handler.(TCPHandler)
	if !ok {
		return nil
	}
	return reqHandle
}

func (tc *tcpconn) getOnClosed() OnTCPClosed {
	onClosed := tc.closeHandle.Load()
	if onClosed == nil {
		return nil
	}
	closeHandle, ok := onClosed.(OnTCPClosed)
	if !ok {
		return nil
	}
	return closeHandle
}

func (tc *tcpconn) refreshConn() error {
	if tc.idleTimer != nil {
		if err := asynctimer.Add(tc.idleTimer); err != nil {
			return err
		}
	}
	return nil
}

func tcpOnIdle(data any) {
	c, ok := data.(Conn)
	if !ok {
		return
	}
	c.Close()
}

func tcpOnRead(data any, ioData *iovec.IOData) error {
	// data passed from desc to tcpOnRead must be of type *tcpconn.
	tc, ok := data.(*tcpconn)
	if !ok || tc == nil {
		return fmt.Errorf("tcpOnRead: invalid data %+v, type %T", tc, tc)
	}
	if !tc.beginJobSafely(sysRead) {
		return nil
	}
	defer tc.endJobSafely(sysRead)

	tc.refreshConn()
	if err := tc.inBuffer.Fill(&tc.nfd, int(tc.waitReadLen.Load()), ioData); err != nil {
		if err == buffer.ErrBufferFull {
			return nil
		}
		return err
	}

	if tc.nonblocking {
		return tcpSyncHandle(tc)
	}
	// wakeup one reading blocked goroutine
	select {
	case tc.readTrigger <- struct{}{}:
	default:
	}
	// sync mode doesn't have onRequest handler
	handler := tc.getOnRequest()
	if handler == nil {
		return nil
	}
	// make sure only one goroutine will process data
	if !tc.reading.TryLock() {
		tc.postpone.IncReadingTryLockFail()
		return nil
	}
	return doTask(tc)
}

func tcpOnWrite(data any) error {
	// data passed from desc to tcpOnWrite must be of type *tcpconn.
	tc, ok := data.(*tcpconn)
	if !ok || tc == nil {
		return fmt.Errorf("tcpOnWrite: invalid data %+v, type %T", tc, tc)
	}
	if !tc.beginJobSafely(sysWrite) {
		return nil
	}
	defer tc.endJobSafely(sysWrite)

	metrics.Add(metrics.TCPOnWriteCalls, 1)
	if err := tc.writeToNetFD(); err != nil {
		if errors.Is(err, unix.EAGAIN) {
			return nil
		}
		return err
	}
	// waiting for next OnWrite Event to write the left data
	if tc.outBuffer.LenRead() != 0 {
		return nil
	}

	if err := tc.nfd.Control(poller.ModReadable); err != nil {
		return err
	}
	tc.writing.Unlock()

	// race condition check, make sure the incoming data in short time between LenRead() and Unlock()
	// can be handled by monitoring OnWrite event.
	if tc.outBuffer.LenRead() != 0 && tc.writing.TryLock() {
		metrics.Add(metrics.TCPWriteNotify, 1)
		return tc.nfd.Control(poller.ModReadWriteable)
	}
	return nil
}

func tcpOnHup(data any) {
	tc, ok := data.(*tcpconn)
	if ok && tc != nil {
		tc.Close()
	}
}

func tcpAsyncHandler(conn *tcpconn) {
	handler := conn.getOnRequest()
	if handler == nil {
		return
	}
	for {
		for conn.Len() > 0 && conn.IsActive() {
			if err := handler(conn); err != nil {
				log.Debugf("tcpAsyncHandler err: %v\n", err)
				conn.reading.Unlock()
				conn.Close()
				return
			}
		}
		conn.reading.Unlock()
		conn.postpone.ResetReadingTryLockFail()
		// check again to prevent packet loss because
		// conn may receive data before Unlock.
		if conn.Len() <= 0 || !conn.reading.TryLock() {
			return
		}
	}
}

func tcpSyncHandle(conn *tcpconn) error {
	handler := conn.getOnRequest()
	if handler == nil {
		return errors.New("no OnRequest handler")
	}
	conn.postpone.ResetLoopCnt()
	for conn.Len() > 0 && conn.IsActive() {
		conn.postpone.IncLoopCnt()
		err := handler(conn)
		if err == nil {
			continue
		}
		if err == EAGAIN {
			return nil
		}
		return err
	}
	conn.postpone.CheckLoopCnt()
	return nil
}

// SetMetaData sets meta data.
func (tc *tcpconn) SetMetaData(m any) {
	tc.metaData = m
}

// GetMetaData gets meta data.
func (tc *tcpconn) GetMetaData() any {
	return tc.metaData
}
