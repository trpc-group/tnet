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

package tls

import (
	"crypto/tls"
	"net"

	"github.com/pkg/errors"
	"trpc.group/trpc-go/tnet"
)

// Handler is the tls connection handler.
type Handler = func(c Conn) error

// NewService creates a new tls service.
func NewService(ln net.Listener, handler Handler, opts ...ServerOption) (tnet.Service, error) {
	var options serverOptions
	for _, opt := range opts {
		opt(&options)
	}
	tnetOpts := []tnet.Option{
		tnet.WithTCPKeepAlive(options.keepAlive),
		tnet.WithTCPIdleTimeout(options.idleTimeout),
		// After sending the data, tls needs to reuse the underlying
		// bytes slice, so SafeWrite must be enabled to ensure that
		// a copy is made when the data is written.
		tnet.WithSafeWrite(true),
		tnet.WithFlushWrite(options.flushWrite),
		tnet.WithOnTCPOpened(func(c tnet.Conn) error {
			tc := &conn{
				Conn: tls.Server(c, options.cfg),
				raw:  c,
			}
			c.SetMetaData(tc)
			if options.onOpened != nil {
				return options.onOpened(tc)
			}
			return nil
		}),
	}
	handleFunc := func(c tnet.Conn) error {
		if c.GetMetaData() == nil {
			return errors.New("metadata is empty, expect tls connection")
		}
		tc, ok := c.GetMetaData().(*conn)
		if !ok {
			return errors.New("tls connection is not stored in metadata")
		}
		// Inside the crypto/tls, there is an internal buffer to store data.
		// When the tnet buffer is empty but there is data present in the crypto/tls buffer,
		// the onRequest function does not trigger. In order to ensure that we can read all
		// the data from connection, we use a for loop here.
		for {
			if !c.IsActive() {
				return tnet.ErrConnClosed
			}
			if err := handler(tc); err != nil {
				return err
			}
		}
	}
	if options.onClosed != nil {
		tnetOpts = append(tnetOpts, tnet.WithOnTCPClosed(func(c tnet.Conn) error {
			if c.GetMetaData() == nil {
				return errors.New("metadata is empty, expect tls connection")
			}
			tc, ok := c.GetMetaData().(*conn)
			if !ok {
				return errors.New("tls connection is not stored in metadata")
			}
			return options.onClosed(tc)
		}))
	}
	return tnet.NewTCPService(ln, handleFunc, tnetOpts...)
}
