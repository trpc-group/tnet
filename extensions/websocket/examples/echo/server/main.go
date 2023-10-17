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

// Package main is the main package.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"time"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/extensions/websocket"
)

var (
	addr      = flag.String("addr", ":9876", "websocket server listen address")
	enableTLS = flag.Bool("enabletls", false, "enable TLS")
	certPath  = flag.String("cert", "../../testdata/server.crt", "server certificate file path")
	keyPath   = flag.String("key", "../../testdata/server.key", "server private key file path")
)

func main() {
	flag.Parse()
	ln, err := tnet.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("listen ", *addr)
	opts := []websocket.ServerOption{websocket.WithIdleTimeout(time.Minute)}
	if *enableTLS {
		cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
		if err != nil {
			log.Fatal(err)
		}
		opts = append(opts, websocket.WithServerTLSConfig(&tls.Config{Certificates: []tls.Certificate{cert}}))
	}
	s, err := websocket.NewService(ln, handler, opts...)
	if err != nil {
		log.Fatal(err)
	}
	log.Print(s.Serve(context.Background()))
}

func handler(c websocket.Conn) error {
	tp, msg, err := c.ReadMessage()
	if err != nil {
		return err
	}
	return c.WriteMessage(tp, msg)
}
