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
	stdtls "crypto/tls"
	"flag"
	"fmt"
	"log"

	"trpc.group/trpc-go/tnet/tls"
)

var (
	addr  = flag.String("addr", "127.0.0.1:9889", "dial address")
	hello = []byte("hello")
)

func main() {
	flag.Parse()
	// InsecureSkipVerify should be used only for testing.
	c, err := tls.Dial("tcp", *addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	if err != nil {
		log.Fatal(err)
	}
	_, err = c.Write(hello)
	if err != nil {
		log.Fatal(err)
	}
	data := make([]byte, 10)
	n, err := c.Read(data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("receive %v\n", string(data[:n]))
}
