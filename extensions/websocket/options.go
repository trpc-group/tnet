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

package websocket

import (
	"context"
	"crypto/tls"
	"time"
)

const defaultTimeout = 10 * time.Second

var defaultServerOptions = serverOptions{
	beforeHandshake:     func(ctx context.Context) (context.Context, error) { return ctx, nil },
	afterHandshake:      func(ctx context.Context, c Conn) error { return nil },
	newHandshakeContext: func() context.Context { return context.Background() },
}

type serverOptions struct {
	beforeHandshake     func(context.Context) (context.Context, error)
	afterHandshake      func(context.Context, Conn) error
	newHandshakeContext func() context.Context
	protocolSelect      func([]byte) bool
	protocolCustom      func([]byte) (string, bool)
	pingHandler         func(Conn, []byte) error
	pongHandler         func(Conn, []byte) error
	messageType         MessageType
	tlsConfig           *tls.Config
	keepAlive           time.Duration
	idleTimeout         time.Duration
	onClosed            func(Conn) error
	combineWrites       bool
}

// ServerOption is the type for a single server option.
type ServerOption func(*serverOptions)

// WithHookBeforeHandshake provides the option to set before handshake procedures.
func WithHookBeforeHandshake(hook func(context.Context) (context.Context, error)) ServerOption {
	return func(o *serverOptions) {
		o.beforeHandshake = hook
	}
}

// WithHookAfterHandshake provides the option to set after handshake procedures.
func WithHookAfterHandshake(hook func(context.Context, Conn) error) ServerOption {
	return func(o *serverOptions) {
		o.afterHandshake = hook
	}
}

// WithNewHandshakeContext provides the handshake context creator function.
func WithNewHandshakeContext(newContext func() context.Context) ServerOption {
	return func(o *serverOptions) {
		o.newHandshakeContext = newContext
	}
}

// WithProtocolSelect provides the option for server to select a
// subprotocol from the subprotocol list requested by the client.
// If this field is set, then the first matched protocol is sent
// to the client as negotiated.
func WithProtocolSelect(protocolSelect func([]byte) bool) ServerOption {
	return func(o *serverOptions) {
		o.protocolSelect = protocolSelect
	}
}

// WithProtocolCustom provides the option for server to parse the
// "Sec-WebSocket-Protocol" header manually. If protocolCustom is
// set, it will be used instead of protocolSelect.
func WithProtocolCustom(protocolCustom func([]byte) (string, bool)) ServerOption {
	return func(o *serverOptions) {
		o.protocolCustom = protocolCustom
	}
}

// WithPingHandler provides the option to set customized Ping frame handler
// for all connections.
func WithPingHandler(handler func(Conn, []byte) error) ServerOption {
	return func(o *serverOptions) {
		o.pingHandler = handler
	}
}

// WithPongHandler provides the option to set customized Pong frame handler
// for all connections.
func WithPongHandler(handler func(Conn, []byte) error) ServerOption {
	return func(o *serverOptions) {
		o.pongHandler = handler
	}
}

// WithServerMessageType provides the option to set message type for each
// connection created by the server.
func WithServerMessageType(tp MessageType) ServerOption {
	return func(o *serverOptions) {
		o.messageType = tp
	}
}

// WithServerTLSConfig provides the option to set TLS configuration.
// To enable TLS, the endpoint must set this option with a non-nil value.
func WithServerTLSConfig(cfg *tls.Config) ServerOption {
	return func(o *serverOptions) {
		o.tlsConfig = cfg
	}
}

// WithTCPKeepAlive sets the tcp keep alive interval.
func WithTCPKeepAlive(keepAlive time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.keepAlive = keepAlive
	}
}

// WithIdleTimeout sets the idle timeout to close the connection.
func WithIdleTimeout(idleTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.idleTimeout = idleTimeout
	}
}

// WithOnClosed registers the OnClosed method that is fired when the connection is closed.
func WithOnClosed(onClosed func(Conn) error) ServerOption {
	return func(o *serverOptions) {
		o.onClosed = onClosed
	}
}

// WithServerCombinedWrites returns an Option to enable/disable combining header and payload writes.
func WithServerCombinedWrites(enabled bool) ServerOption {
	return func(o *serverOptions) {
		o.combineWrites = enabled
	}
}

type clientOptions struct {
	timeout       time.Duration
	subprotocols  []string
	messageType   MessageType
	tlsConfig     *tls.Config
	combineWrites bool
}

func (o *clientOptions) setDefaults() {
	o.timeout = defaultTimeout
}

// ClientOption is the type for a single dial option.
type ClientOption func(*clientOptions)

// WithTimeout provides the option to set dial timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.timeout = timeout
	}
}

// WithSubProtocols provides the option to set sub protocols.
func WithSubProtocols(subprotocols []string) ClientOption {
	return func(o *clientOptions) {
		o.subprotocols = subprotocols
	}
}

// WithClientMessageType provides the option to set message type for each
// connection created by the client.
func WithClientMessageType(tp MessageType) ClientOption {
	return func(o *clientOptions) {
		o.messageType = tp
	}
}

// WithClientTLSConfig provides the option to set TLS configuration.
// To enable TLS, the endpoint must set this option with a non-nil value.
func WithClientTLSConfig(cfg *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.tlsConfig = cfg
	}
}

// WithClientCombinedWrites returns an Option to enable/disable combining header and payload writes.
func WithClientCombinedWrites(enabled bool) ClientOption {
	return func(o *clientOptions) {
		o.combineWrites = enabled
	}
}
