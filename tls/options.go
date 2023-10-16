// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package tls

import (
	"crypto/tls"
	"time"
)

const defaultDialTimeout = 10 * time.Second

type serverOptions struct {
	cfg         *tls.Config
	keepAlive   time.Duration
	idleTimeout time.Duration
	flushWrite  bool
	onOpened    OnOpened
	onClosed    OnClosed
}

// OnOpened fires when the tcp connection is established.
type OnOpened func(conn Conn) error

// OnClosed fires when the connection is closed.
// In this method, please do not perform read-write operations, because the connection has been closed.
// But you can still manipulate the MetaData in the connection.
type OnClosed func(conn Conn) error

// ServerOption is the type for a single server option.
type ServerOption func(*serverOptions)

// WithServerTLSConfig provides the option to set TLS configuration.
func WithServerTLSConfig(cfg *tls.Config) ServerOption {
	return func(o *serverOptions) {
		o.cfg = cfg
	}
}

// WithTCPKeepAlive sets the tcp keep alive interval.
func WithTCPKeepAlive(keepAlive time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.keepAlive = keepAlive
	}
}

// WithServerIdleTimeout sets the idle timeout to close the connection.
func WithServerIdleTimeout(idleTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.idleTimeout = idleTimeout
	}
}

// WithServerFlushWrite sets the flush write flag for server connection.
func WithServerFlushWrite(flushWrite bool) ServerOption {
	return func(o *serverOptions) {
		o.flushWrite = flushWrite
	}
}

// WithOnOpened registers the OnOpened method that is fired when connection is established.
func WithOnOpened(onOpened OnOpened) ServerOption {
	return func(o *serverOptions) {
		o.onOpened = onOpened
	}
}

// WithOnClosed registers the OnClosed method that is fired when connection is closed.
func WithOnClosed(onClosed OnClosed) ServerOption {
	return func(o *serverOptions) {
		o.onClosed = onClosed
	}
}

type clientOptions struct {
	cfg         *tls.Config
	dialTimeout time.Duration
	idleTimeout time.Duration
	flushWrite  bool
}

// ClientOption is the type for a single client option.
type ClientOption func(*clientOptions)

func (o *clientOptions) setDefaults() {
	o.dialTimeout = defaultDialTimeout
}

// WithTimeout provides the option to set dial timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.dialTimeout = timeout
	}
}

// WithClientTLSConfig provides the option to set TLS configuration.
func WithClientTLSConfig(cfg *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.cfg = cfg
	}
}

// WithClientFlushWrite sets the flush write flag for client connection.
func WithClientFlushWrite(flushWrite bool) ClientOption {
	return func(o *clientOptions) {
		o.flushWrite = flushWrite
	}
}

// WithClientIdleTimeout sets the idle timeout to close the connection.
func WithClientIdleTimeout(idleTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.idleTimeout = idleTimeout
	}
}
