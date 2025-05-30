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
	"time"

	"go.uber.org/atomic"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/autopostpone"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/mcache"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/locker"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/poller"
	"trpc.group/trpc-go/tnet/internal/timer"
	"trpc.group/trpc-go/tnet/metrics"
)

// udpconn must implements Conn interface.
var _ PacketConn = (*udpconn)(nil)

type udpconn struct {
	metaData    interface{}
	reqHandle   atomic.Value
	closeHandle atomic.Value
	readTrigger chan struct{}
	inBuffer    buffer.Buffer
	outBuffer   buffer.Buffer
	rtimer      *timer.Timer
	wtimer      *timer.Timer
	nfd         netFD

	closer
	postpone     autopostpone.PostponeWrite
	reading      locker.Locker
	writing      locker.Locker
	nonblocking  bool
	closeService *sync.WaitGroup
}

func (uc *udpconn) schedule() error {
	return uc.nfd.Schedule(udpOnRead, udpOnWrite, nil, uc)
}

func (uc *udpconn) waitRead() error {
	if !uc.IsActive() {
		return ErrConnClosed
	}
	if uc.inBuffer.LenRead() > 0 {
		return nil
	}
	if uc.nonblocking {
		return EAGAIN
	}
	if uc.rtimer != nil && !uc.rtimer.IsZero() {
		return uc.waitReadWithTimeout()
	}

	for uc.inBuffer.LenRead() == 0 {
		if !uc.IsActive() {
			return ErrConnClosed
		}
		<-uc.readTrigger
	}
	return nil
}

func (uc *udpconn) waitReadWithTimeout() error {
	uc.rtimer.Start()
	select {
	case <-uc.rtimer.Wait():
		return uc.errTimeout()
	default:
	}
	for uc.inBuffer.LenRead() == 0 {
		if !uc.IsActive() {
			return ErrConnClosed
		}
		select {
		case <-uc.readTrigger:
			continue
		case <-uc.rtimer.Wait():
			return uc.errTimeout()
		}
	}
	return nil
}

type packet struct {
	block []byte
}

// Data returns the data of the packet.
func (p *packet) Data() ([]byte, error) {
	return getUDPData(p.block)
}

// Free will release the underlying buffer.
func (p *packet) Free() {
	mcache.Free(p.block)
}

// ReadPacket reads a packet from the connection, without copying the underlying buffer.
func (uc *udpconn) ReadPacket() (Packet, net.Addr, error) {
	if !uc.beginJobSafely(apiRead) {
		return nil, nil, ErrConnClosed
	}
	defer uc.endJobSafely(apiRead)

	block, err := uc.readBlock()
	if err != nil {
		return nil, nil, err
	}
	defer uc.inBuffer.Release()

	addr, err := getUDPAddr(block)
	if err != nil {
		return nil, nil, err
	}

	p := &packet{block: block}
	return p, addr, err
}

// ReadFrom reads data from the udpconn.
func (uc *udpconn) ReadFrom(b []byte) (int, net.Addr, error) {
	if len(b) == 0 {
		return 0, nil, nil
	}
	if !uc.beginJobSafely(apiRead) {
		return 0, nil, ErrConnClosed
	}
	defer uc.endJobSafely(apiRead)

	block, err := uc.readBlock()
	if err != nil {
		return 0, nil, err
	}
	defer mcache.Free(block)
	defer uc.inBuffer.Release()

	s, addr, err := getUDPDataAndAddr(block)
	if err != nil {
		return 0, nil, err
	}

	nc := copy(b, s)
	return nc, addr, err
}

func (uc *udpconn) readBlock() ([]byte, error) {
	if err := uc.waitRead(); err != nil {
		return nil, err
	}
	block, err := uc.inBuffer.ReadBlock()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (uc *udpconn) errTimeout() error {
	err := fmt.Errorf("write udp %s: i/o timeout",
		uc.LocalAddr().String())
	return netError{error: err, isTimeout: true}

}

// WriteTo writes a packet with payload p to addr.
func (uc *udpconn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if uc.wtimer != nil && uc.wtimer.Expired() {
		err := uc.errTimeout()
		return 0, netError{error: err, isTimeout: true}
	}
	if !uc.beginJobSafely(apiWrite) {
		return 0, ErrConnClosed
	}
	defer uc.endJobSafely(apiWrite)
	if uc.postpone.Enabled() {
		return uc.writeToBuffer(p, addr)
	}
	n, err := uc.writeToNetFD(p, addr)
	if (n == 0 && err == nil) || errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
		return uc.writeToBuffer(p, addr)
	}
	return n, err
}

func (uc *udpconn) writeToNetFD(p []byte, addr net.Addr) (int, error) {
	if !uc.writing.TryLock() {
		return 0, nil
	}
	n, err := uc.nfd.WriteTo(p, addr)
	metrics.Add(metrics.UDPWriteToCalls, 1)
	uc.writing.Unlock()
	if err != nil {
		metrics.Add(metrics.UDPWriteToFails, 1)
		return n, err
	}
	if uc.outBuffer.LenRead() != 0 && uc.writing.TryLock() {
		err = uc.nfd.Control(poller.ModReadWriteable)
	}
	return n, err
}

func (uc *udpconn) writeToBuffer(p []byte, addr net.Addr) (int, error) {
	block, err := parcel(p, addr)
	if err != nil {
		return 0, err
	}
	n := uc.outBuffer.Write(false, block)
	n -= netutil.SockaddrSize
	if !uc.writing.TryLock() {
		return n, nil
	}
	if uc.outBuffer.LenRead() != 0 {
		return n, uc.nfd.Control(poller.ModReadWriteable)
	}
	uc.writing.Unlock()
	if uc.outBuffer.LenRead() != 0 && uc.writing.TryLock() {
		err = uc.nfd.Control(poller.ModReadWriteable)
	}
	return n, err
}

// Close closes the connection.
func (uc *udpconn) Close() error {
	if !uc.beginJobSafely(closeAll) {
		return nil
	}
	defer uc.endJobSafely(closeAll)
	uc.closeJobSafely(sysRead)
	close(uc.readTrigger)
	uc.closeAllJobs()

	if onClosed := uc.getOnClosed(); onClosed != nil {
		onClosed(uc)
	}
	uc.metaData = nil
	if uc.rtimer != nil {
		uc.rtimer.Stop()
	}
	if uc.wtimer != nil {
		uc.wtimer.Stop()
	}
	if uc.closeService != nil {
		uc.closeService.Done()
	}
	uc.nfd.close()
	return nil
}

// IsActive checks whether the udpconn is active or not.
func (uc *udpconn) IsActive() bool {
	return !uc.closed()
}

// Read reads data from the udpconn.
func (uc *udpconn) Read(b []byte) (int, error) {
	n, _, err := uc.ReadFrom(b)
	return n, err
}

// Write writes data to the connection.
func (uc *udpconn) Write(b []byte) (int, error) {
	addr := uc.RemoteAddr()
	if addr == nil {
		return 0, errors.New("miss remote address")
	}
	if uc.wtimer != nil && uc.wtimer.Expired() {
		err := uc.errTimeout()
		return 0, netError{error: err, isTimeout: true}
	}
	if !uc.beginJobSafely(apiWrite) {
		return 0, ErrConnClosed
	}
	defer uc.endJobSafely(apiWrite)
	return unix.Write(uc.nfd.fd, b)
}

// LocalAddr returns the local network address.
func (uc *udpconn) LocalAddr() net.Addr {
	return uc.nfd.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (uc *udpconn) RemoteAddr() net.Addr {
	return uc.nfd.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
// A zero value for t means I/O operations will not time out.
func (uc *udpconn) SetDeadline(t time.Time) error {
	if err := uc.SetReadDeadline(t); err != nil {
		return err
	}
	return uc.SetWriteDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// A zero value for t means Read will not time out.
func (uc *udpconn) SetReadDeadline(t time.Time) error {
	if !uc.IsActive() {
		return ErrConnClosed
	}
	if uc.rtimer == nil {
		uc.rtimer = timer.New(t)
		return nil
	}
	uc.rtimer.Reset(t)
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// A zero value for t means Write will not time out.
func (uc *udpconn) SetWriteDeadline(t time.Time) error {
	if !uc.IsActive() {
		return ErrConnClosed
	}
	if uc.wtimer == nil {
		uc.wtimer = timer.New(t)
		return nil
	}
	uc.wtimer.Reset(t)
	return nil
}

// SetMaxPacketSize sets maximal UDP packet size when receiving UDP packets.
func (uc *udpconn) SetMaxPacketSize(size int) {
	uc.nfd.udpBufferSize = size
}

// SetExactUDPBufferSizeEnabled set whether to allocate an exact-sized buffer for UDP packets, false in default.
// If set to true, an exact-sized buffer is allocated for each UDP packet, requiring two system calls.
// If set to false, a fixed buffer size of maxUDPPacketSize is used, 65536 in default, requiring only one system call.
// This option should be used in conjunction with the ReadPacket method to properly read UDP packets.
func (uc *udpconn) SetExactUDPBufferSizeEnabled(exactUDPBufferSizeEnabled bool) {
	uc.nfd.exactUDPBufferSizeEnabled = exactUDPBufferSizeEnabled
}

// SetNonBlocking set conn to nonblocking. Read APIs will return EAGAIN when there is no
// enough data for reading.
func (uc *udpconn) SetNonBlocking(nonblock bool) {
	uc.nonblocking = nonblock
}

// SetFlushWrite sets whether to flush the data or not.
// Default behavior is notify.
// Deprecated: whether enable this feature is controlled by system automatically.
func (uc *udpconn) SetFlushWrite(flushWrite bool) {}

// Len returns the total length of the readable data in the reader.
func (uc *udpconn) Len() int {
	if !uc.beginJobSafely(apiCtrl) {
		return 0
	}
	defer uc.endJobSafely(apiCtrl)
	return uc.inBuffer.LenRead()
}

// SetOnClosed sets the additional close process for a connection.
// Handle is executed when the connection is closed.
func (uc *udpconn) SetOnClosed(handle OnUDPClosed) error {
	if !uc.IsActive() {
		return ErrConnClosed
	}
	if handle == nil {
		return errors.New("onClosed can't be nil")
	}
	uc.closeHandle.Store(handle)
	return nil
}

func (uc *udpconn) getOnClosed() OnUDPClosed {
	onClosed := uc.closeHandle.Load()
	if onClosed == nil {
		return nil
	}
	closeHandle, ok := onClosed.(OnUDPClosed)
	if !ok {
		return nil
	}
	return closeHandle
}

// SetOnRequest can set or replace the UDPHandler method for a connection
// Generally, on the server side which is set when the connection is established.
// On the client side, if necessary, make sure that UDPHandler is set before sending data.
func (uc *udpconn) SetOnRequest(handle UDPHandler) error {
	if handle == nil {
		return errors.New("handle can't be nil")
	}
	uc.reqHandle.Store(handle)
	return nil
}

func (uc *udpconn) getOnRequest() UDPHandler {
	handler := uc.reqHandle.Load()
	if handler == nil {
		return nil
	}
	reqHandle, ok := handler.(UDPHandler)
	if !ok {
		return nil
	}
	return reqHandle
}

func udpOnRead(data interface{}, _ *iovec.IOData) error {
	// Data passed from desc to udpOnRead must be of type *udpconn.
	uc, ok := data.(*udpconn)
	if !ok || uc == nil {
		return fmt.Errorf("udpOnRead: invalid data %+v, type %T", uc, uc)
	}
	if !uc.beginJobSafely(sysRead) {
		return nil
	}
	defer uc.endJobSafely(sysRead)

	if err := uc.nfd.FillToBuffer(&uc.inBuffer); err != nil {
		return err
	}
	if uc.nonblocking {
		return udpSyncHandle(uc)
	}
	// Wake up one reading blocked goroutine.
	select {
	case uc.readTrigger <- struct{}{}:
	default:
	}
	// Sync mode doesn't have onRequest handler.
	handler := uc.getOnRequest()
	if handler == nil {
		return nil
	}
	// Make sure only one goroutine will process data.
	if !uc.reading.TryLock() {
		uc.postpone.IncReadingTryLockFail()
		return nil
	}
	return doTask(uc)
}

func udpOnWrite(data interface{}) error {
	// Data passed from desc to udpOnWrite must be of type *udpconn.
	uc, ok := data.(*udpconn)
	if !ok || uc == nil {
		return fmt.Errorf("udpOnWrite: invalid data %+v, type %T", uc, uc)
	}
	if !uc.beginJobSafely(sysWrite) {
		return nil
	}
	defer uc.endJobSafely(sysWrite)

	if err := uc.nfd.SendPackets(&uc.outBuffer); err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil
		}
		return err
	}
	if uc.outBuffer.LenRead() != 0 {
		return nil
	}
	if err := uc.nfd.Control(poller.ModReadable); err != nil {
		return err
	}
	uc.writing.Unlock()

	// Race condition check, make sure the income data in short time between LenRead() and Unlock()
	// can be handled by monitoring OnWrite event.
	if uc.outBuffer.LenRead() != 0 && uc.writing.TryLock() {
		return uc.nfd.Control(poller.ModReadWriteable)
	}
	return nil
}

func getUDPData(block []byte) ([]byte, error) {
	if len(block) < netutil.SockaddrSize {
		return nil, errors.New("invalid UDP packet")
	}
	buf := block[netutil.SockaddrSize:]
	return buf, nil
}

func getUDPAddr(block []byte) (net.Addr, error) {
	if len(block) < netutil.SockaddrSize {
		return nil, errors.New("invalid UDP packet")
	}
	sockaddr := block[:netutil.SockaddrSize]
	addr, err := netutil.SockaddrSliceToUDPAddr(sockaddr)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func getUDPDataAndAddr(block []byte) ([]byte, net.Addr, error) {
	addr, err := getUDPAddr(block)
	if err != nil {
		return nil, nil, err
	}
	buf, err := getUDPData(block)
	if err != nil {
		return nil, nil, err
	}
	return buf, addr, nil
}

func parcel(buf []byte, addr net.Addr) ([]byte, error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return nil, errors.New("only UDPAddr can be parceled")
	}
	sockaddr, err := netutil.UDPAddrToSockaddrSlice(udpAddr)
	if err != nil {
		return nil, err
	}
	block := append(sockaddr, buf...)
	return block, nil
}

func udpAsyncHandler(conn *udpconn) {
	handler := conn.getOnRequest()
	if handler == nil {
		return
	}
	for {
		for conn.Len() > 0 && conn.IsActive() {
			if err := handler(conn); err != nil {
				conn.reading.Unlock()
				conn.Close()
				return
			}
		}
		conn.reading.Unlock()
		conn.postpone.ResetReadingTryLockFail()
		// Check again to prevent packet loss because conn may receive data before Unlock.
		if conn.Len() <= 0 || !conn.reading.TryLock() {
			return
		}
	}
}

func udpSyncHandle(conn *udpconn) error {
	handler := conn.getOnRequest()
	if handler == nil {
		panic("nonblocking mode must set handler")
	}
	conn.postpone.ResetLoopCnt()
	for conn.Len() > 0 && conn.IsActive() {
		conn.postpone.IncLoopCnt()
		if err := handler(conn); err != nil {
			conn.Close()
			return err
		}
	}
	conn.postpone.CheckLoopCnt()
	return nil
}

// SetMetaData sets meta data.
func (uc *udpconn) SetMetaData(m interface{}) {
	uc.metaData = m
}

// GetMetaData gets meta data.
func (uc *udpconn) GetMetaData() interface{} {
	return uc.metaData
}
