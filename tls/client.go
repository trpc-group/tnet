// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package tls

import (
	"context"
	"crypto/tls"
	"strings"

	"trpc.group/trpc-go/tnet"
)

// Dial creates a client connection.
func Dial(network, addr string, opts ...ClientOption) (Conn, error) {
	var options clientOptions
	options.setDefaults()
	for _, opt := range opts {
		opt(&options)
	}
	config := fixConfig(options.cfg, addr)
	return dial(network, addr, &options, config)
}

func fixConfig(config *tls.Config, addr string) *tls.Config {
	if config == nil {
		config = &tls.Config{}
	}
	if config.ServerName != "" {
		return config
	}
	// Make a copy to avoid polluting argument or default.
	c := config.Clone()
	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]
	c.ServerName = hostname
	return c
}

func dial(network, addr string, options *clientOptions, config *tls.Config) (Conn, error) {
	rawConn, err := tnet.DialTCP(network, addr, options.dialTimeout)
	if err != nil {
		return nil, err
	}
	// SafeWrite must be enabled for TLS.
	rawConn.SetSafeWrite(true)
	tlsConn := tls.Client(rawConn, config)
	ctx, cancel := context.WithTimeout(context.Background(), options.dialTimeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, err
	}
	// SetFlushWrite flag after handshake.
	rawConn.SetFlushWrite(options.flushWrite)
	rawConn.SetIdleTimeout(options.idleTimeout)
	return &conn{Conn: tlsConn, raw: rawConn}, nil
}
