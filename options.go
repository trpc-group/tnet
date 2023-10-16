// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package tnet

import (
	"time"

	"trpc.group/trpc-go/tnet/internal/poller"
)

// SetNumPollers is used to set the number of pollers. Generally it is not actively used.
// Note that n can't be smaller than the current poller numbers.
//
// NOTE: the default poller number is 1.
func SetNumPollers(n int) error {
	return poller.SetNumPollers(n)
}

// NumPollers returns the current number of pollers.
func NumPollers() int {
	return poller.NumPollers()
}

// EnablePollerGoschedAfterEvent enables calling runtime.Gosched() after processing of each event
// during epoll wait handling.
// This function can only be called inside func init().
func EnablePollerGoschedAfterEvent() {
	poller.GoschedAfterEvent = true
}

// OnTCPOpened fires when the tcp connection is established.
type OnTCPOpened func(conn Conn) error

// OnTCPClosed fires when the tcp connection is closed.
// In this method, please do not perform read-write operations, because the connection has been closed.
// But you can still manipulate the MetaData in the connection.
type OnTCPClosed func(conn Conn) error

// OnUDPClosed fires when the udp connection is closed.
// In this method, please do not perform read-write operations, because the connection has been closed.
// But you can still manipulate the MetaData in the connection.
type OnUDPClosed func(conn PacketConn) error

// TCPHandler fires when the tcp connection receives data.
type TCPHandler func(conn Conn) error

// UDPHandler fires when the udp connection receives data.
type UDPHandler func(conn PacketConn) error

// Option tnet service option.
type Option struct {
	f func(*options)
}

type options struct {
	onTCPOpened      OnTCPOpened
	onTCPClosed      OnTCPClosed
	onUDPClosed      OnUDPClosed
	tcpKeepAlive     time.Duration
	tcpIdleTimeout   time.Duration
	nonblocking      bool
	safeWrite        bool
	maxUDPPacketSize int
}

func (o *options) setDefault() {
	o.tcpKeepAlive = defaultTCPKeepAlive
	o.maxUDPPacketSize = defaultUDPBufferSize
}

// WithTCPKeepAlive sets the tcp keep alive interval.
func WithTCPKeepAlive(keepAlive time.Duration) Option {
	return Option{func(op *options) {
		op.tcpKeepAlive = keepAlive
	}}
}

// WithTCPIdleTimeout sets the idle timeout to close tcp connection.
func WithTCPIdleTimeout(idleTimeout time.Duration) Option {
	return Option{func(op *options) {
		op.tcpIdleTimeout = idleTimeout
	}}
}

// WithOnTCPOpened registers the OnTCPOpened method that is fired when connection is established.
func WithOnTCPOpened(onTCPOpened OnTCPOpened) Option {
	return Option{func(op *options) {
		op.onTCPOpened = onTCPOpened
	}}
}

// WithOnTCPClosed registers the OnTCPClosed method that is fired when tcp connection is closed.
func WithOnTCPClosed(onTCPClosed OnTCPClosed) Option {
	return Option{func(op *options) {
		op.onTCPClosed = onTCPClosed
	}}
}

// WithOnUDPClosed registers the OnUDPClosed method that is fired when udp connection is closed.
func WithOnUDPClosed(onUDPClosed OnUDPClosed) Option {
	return Option{func(op *options) {
		op.onUDPClosed = onUDPClosed
	}}
}

// WithNonBlocking set conn/packconn to nonblocking mode
func WithNonBlocking(nonblock bool) Option {
	return Option{func(op *options) {
		op.nonblocking = nonblock
	}}
}

// WithTCPFlushWrite sets whether use flush write for TCP
// connection or not. Default value is false.
// Deprecated: whether enable this feature is controlled by system automatically.
func WithTCPFlushWrite(flush bool) Option {
	return Option{func(op *options) {}}
}

// WithFlushWrite sets whether use flush write for TCP and UDP
// connection or not. Default value is false.
// Deprecated: whether enable this feature is controlled by system automatically.
func WithFlushWrite(flush bool) Option {
	return Option{func(op *options) {}}
}

// WithSafeWrite sets the value of safeWrite for TCP.
// Default value is false.
//
// This option affects the behavior of Write/Writev.
//
//	If safeWrite = false: the lifetime of buffers passed into Write/Writev will
//	  be handled by tnet, which means users cannot reuse the buffers after passing
//	  them into Write/Writev.
//	If safeWrite = true: the given buffers is copied into tnet's own buffer.
//	  Therefore users can reuse the buffers passed into Write/Writev.
func WithSafeWrite(safeWrite bool) Option {
	return Option{func(op *options) {
		op.safeWrite = safeWrite
	}}
}

// WithMaxUDPPacketSize sets maximal UDP packet size when receiving UDP packets.
func WithMaxUDPPacketSize(size int) Option {
	return Option{func(op *options) {
		op.maxUDPPacketSize = size
	}}
}
