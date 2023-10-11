// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package websocket provides websocket connection interface.
package websocket

import (
	"io"
	"net"
	"time"
)

// MessageType specifies message types.
type MessageType int

// Message types.
const (
	Text MessageType = iota + 1
	Binary
	Ping
	Pong
	Close
)

// String implements fmt.Stringer.
func (m MessageType) String() string {
	switch m {
	case Text:
		return "Text"
	case Binary:
		return "Binary"
	case Ping:
		return "Ping"
	case Pong:
		return "Pong"
	case Close:
		return "Close"
	default:
		return "Invalid"
	}
}

// Conn provides websocket connection interface.
type Conn interface {
	net.Conn
	// ReadMessage reads data message.
	ReadMessage() (MessageType, []byte, error)
	// NextMessageReader returns a reader to read the next message.
	NextMessageReader() (MessageType, io.Reader, error)
	// WriteMessage writes message in a single frame.
	WriteMessage(MessageType, []byte) error
	// WritevMessage writes multiple byte slices as a message in a single frame.
	WritevMessage(MessageType, ...[]byte) error
	// NextMessageWriter return a writer to write the next message.
	// A finished message write should end with writer.Close().
	NextMessageWriter(MessageType) (io.WriteCloser, error)
	// SetMetaData sets metadata. Through this method, users can bind some custom data to a connection.
	SetMetaData(interface{})
	// GetMetaData gets meta data.
	GetMetaData() interface{}
	// Subprotocol returns the negotiated protocol for the connection.
	Subprotocol() string
	// SetPingHandler sets customized Ping frame handler.
	SetPingHandler(handler func(Conn, []byte) error)
	// SetPongHandler sets customized Pong frame handler.
	SetPongHandler(handler func(Conn, []byte) error)
	// SetIdleTimeout sets connection level idle timeout.
	SetIdleTimeout(time.Duration) error
	// SetOnRequest sets request handler for websocket connection.
	// Typically used by websocket client.
	SetOnRequest(handle Handler) error
	// SetOnClosed set on closed function for websocket connection.
	SetOnClosed(handle OnClosed) error
}
