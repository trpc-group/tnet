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

// Package main is the main package.
package main

import (
	"context"
	stdtls "crypto/tls"
	"flag"
	"fmt"
	"log"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/tls"
)

var (
	addr     = flag.String("addr", ":9889", "server listen address")
	certPath = flag.String("cert", "../../testdata/server.crt", "server certificate file path")
	keyPath  = flag.String("key", "../../testdata/server.key", "server private key file path")
	buf      = make([]byte, 100)
)

func main() {
	flag.Parse()
	fmt.Println("listen ", *addr)
	// Load public/private key pair from a pair of files.
	cert, err := stdtls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatal(err)
	}
	// Create a listener.
	ln, err := tnet.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	// Create a service.
	s, err := tls.NewService(ln, handler, tls.WithServerTLSConfig(
		&stdtls.Config{Certificates: []stdtls.Certificate{cert}}))
	if err != nil {
		log.Fatal(err)
	}
	log.Println(s.Serve(context.Background()))
}

func handler(c tls.Conn) error {
	n, err := c.Read(buf)
	if err != nil {
		return err
	}
	_, err = c.Write(buf[:n])
	return err
}
