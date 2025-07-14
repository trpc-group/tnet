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
	stdtls "crypto/tls"
	"fmt"
	neturl "net/url"
	"strings"

	"github.com/gobwas/ws"
	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/tls"
)

// Dial creates a client side connection of websocket.
// url is like: "ws://hostname:port/xxx".
func Dial(url string, opts ...ClientOption) (Conn, error) {
	u, err := neturl.ParseRequestURI(url)
	if err != nil {
		return nil, err
	}
	var options clientOptions
	options.setDefaults()
	for _, opt := range opts {
		opt(&options)
	}
	c, err := dial(u, &options)
	dialer := ws.Dialer{
		Protocols: options.subprotocols,
	}
	_, handshake, err := dialer.Upgrade(c, u)
	if err != nil {
		return nil, err
	}
	wc := &conn{
		raw:           c,
		role:          ws.StateClientSide,
		subprotocol:   handshake.Protocol,
		messageType:   options.messageType,
		combineWrites: options.combineWrites,
	}
	c.SetMetaData(wc)
	return wc, nil
}

func dial(u *neturl.URL, options *clientOptions) (rawConnection, error) {
	const (
		httpPort  = ":80"
		httpsPort = ":443"
	)
	switch u.Scheme {
	case "ws":
		_, addr := createHostPortAddress(u.Host, httpPort)
		c, err := tnet.DialTCP("tcp", addr, options.timeout)
		if err != nil {
			return nil, err
		}
		c.SetFlushWrite(true) // Enable flush write for websocket.
		return c, nil
	case "wss":
		hostname, addr := createHostPortAddress(u.Host, httpsPort)
		if options.tlsConfig == nil {
			options.tlsConfig = &stdtls.Config{}
		}
		if options.tlsConfig.ServerName == "" {
			options.tlsConfig.ServerName = hostname
		}
		c, err := tls.Dial("tcp", addr, tls.WithClientTLSConfig(options.tlsConfig))
		if err != nil {
			return nil, err
		}
		c.SetFlushWrite(true) // Enable flush write for websocket.
		return &rawConn{Conn: c}, nil
	default:
		return nil, fmt.Errorf("unexpected websocket scheme: %q", u.Scheme)
	}
}

func createHostPortAddress(host string, defaultPort string) (hostname string, addr string) {
	// Handle ipv6 case like:
	// ldap://[2001:db8::7]:89/
	var (
		colon   = strings.LastIndexByte(host, ':')
		bracket = strings.IndexByte(host, ']')
	)
	if colon > bracket {
		return host[:colon], host
	}
	return host, host + defaultPort
}
