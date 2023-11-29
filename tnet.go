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

// Package tnet provides event loop networking framework.
package tnet

import (
	"context"
	"fmt"
	"net"
	"time"
)

// BaseConn is common for stream and packet oriented network connection.
type BaseConn interface {
	// Conn extends net.Conn, just for interface compatibility.
	net.Conn

	// Len returns the total length of the readable data in the reader.
	Len() int

	// IsActive checks whether the connection is active or not.
	IsActive() bool

	// SetNonBlocking sets conn to nonblocking. Read APIs will return EAGAIN when there is no
	// enough data for reading
	SetNonBlocking(nonblock bool)

	// SetFlushWrite sets whether to flush the data or not.
	// Default value is false.
	// Deprecated: whether enable this feature is controlled by system automatically.
	SetFlushWrite(flushWrite bool)

	// SetMetaData sets metadata. Through this method, users can bind some custom data to a connection.
	SetMetaData(m any)

	// GetMetaData gets metadata.
	GetMetaData() any
}

// Conn is generic for stream oriented network connection.
type Conn interface {
	BaseConn

	// Peek returns the next n bytes without advancing the reader. It waits until it has
	// read at least n bytes or error occurs such as connection closed or read timeout.
	// The bytes stop being valid at the next ReadN or Release call.
	// Zero-Copy API.
	Peek(n int) ([]byte, error)

	// Next returns the next n bytes with advancing the reader, It waits until it has
	// read at least n bytes or error occurs such as connection closed or read timeout.
	// The bytes stop being valid at the next ReadN or Release call.
	// Zero-Copy API.
	Next(n int) ([]byte, error)

	// Skip the next n bytes and advance the reader. It waits until the underlayer has at
	// least n bytes or error occurs such as connection closed or read timeout.
	// Zero-Copy API.
	Skip(n int) error

	// Release releases underlayer buffer when using Peek() and Skip() Zero-Copy APIs.
	Release()

	// ReadN is similar to Peek(), except that it will copy the n bytes data from the underlayer,
	// and advance the reader.
	ReadN(n int) ([]byte, error)

	// Writev provides multiple data slice write in order.
	// The default behavior of Write/Writev will hold a reference to the given byte slices p,
	// therefore if the caller want to reuse byte slice p after calling Write/Writev, the
	// SetSafeWrite(true) option is required.
	Writev(p ...[]byte) (int, error)

	// SetKeepAlive sets keep alive time for tcp connection.
	// By default, keep alive is turned on with value defaultKeepAlive.
	// If keepAlive <= 0, keep alive will be turned off.
	// Otherwise, keep alive value will be round up to seconds.
	SetKeepAlive(t time.Duration) error

	// SetOnRequest can set or replace the TCPHandler method for a connection.
	// Generally, on the server side the handler is set when the connection is established.
	// On the client side, if necessary, make sure that TCPHandler is set before sending data.
	SetOnRequest(handle TCPHandler) error

	// SetOnClosed sets the additional close process for a connection.
	// Handle is executed when the connection is closed.
	SetOnClosed(handle OnTCPClosed) error

	// SetIdleTimeout sets the idle timeout to close connection.
	SetIdleTimeout(d time.Duration) error

	// SetSafeWrite sets whether writing on connection is safe or not.
	// Default is unsafe.
	//
	// This option affects the behavior of Write/Writev.
	//   If safeWrite = false: the lifetime of buffers passed into Write/Writev will
	//     be handled by tnet, which means users cannot reuse the buffers after passing
	//     them into Write/Writev.
	//   If safeWrite = true: the given buffers is copied into tnet's own buffer.
	//     Therefore users can reuse the buffers passed into Write/Writev.
	SetSafeWrite(safeWrite bool)
}

// Service provides startup method to udp/tcp server.
type Service interface {
	// Serve registers a listener and runs blockingly to provide service, including listening to ports,
	// accepting connections and reading trans data.
	// Param ctx is used to shutdown the service with all connections gracefully.
	Serve(ctx context.Context) error
}

// Listen announces on the local network address.
// The network must be "tcp", "tcp4", "tcp6".
func Listen(network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return listenTCP(network, address)
	default:
		return nil, fmt.Errorf("network %s is not support", network)
	}
}

// PacketConn is generic for packet oriented network connection.
type PacketConn interface {
	BaseConn

	// PacketConn extends net.PacketConn, just for interface compatibility.
	net.PacketConn

	// ReadPacket reads a packet from the connection, without copying the underlying buffer.
	// Get the actual data of packet by Packet.Data().
	// Please call Packet.Free() when it is unused, free will recycle the underlying buffer
	// for better performance.
	// Zero-copy API
	ReadPacket() (Packet, net.Addr, error)

	// SetMaxPacketSize sets maximal UDP packet size when receiving UDP packets.
	SetMaxPacketSize(size int)

	// SetOnRequest can set or replace the UDPHandler method for a connection.
	// However, the handler can't be set to nil.
	// Generally, on the server side the handler is set when the connection is established.
	// On the client side, if necessary, make sure that UDPHandler is set before sending data.
	SetOnRequest(handle UDPHandler) error

	// SetOnClosed sets the additional close process for a connection.
	// Handle is executed when the connection is closed.
	SetOnClosed(handle OnUDPClosed) error
}

// Packet represents a UDP packet, created by PacketConn Zero-Copy API ReadPacket.
type Packet interface {
	// Data returns the data of the packet.
	Data() ([]byte, error)

	// Free will release the underlying buffer.
	// It will recycle the underlying buffer for better performance.
	// The bytes will be invalid after free, so free it only when it is no longer in use.
	Free()
}

// ListenPackets announces on the local network address. Reuseport sets whether to enable
// reuseport when creating PacketConns, it will return multiple PacketConn if reuseprot is true.
// Generally, enabling reuseport can make effective use of multi-core and improve performance.
func ListenPackets(network, address string, reuseport bool) ([]PacketConn, error) {
	return listenUDP(network, address, reuseport)
}
