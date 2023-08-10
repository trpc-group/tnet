// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package main is the main package.
package main

import (
	"fmt"
	"log"
	"time"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/examples/udp"
)

func main() {
	// Create a client to connect to the server.
	client, err := tnet.DialUDP("udp", udp.Address, time.Second)
	if err != nil {
		log.Fatal(err)
	}
	// Write data to the connection.
	_, err = client.Write([]byte(udp.Data))
	if err != nil {
		log.Fatal(err)
	}

	// Read data from the connection.
	b := make([]byte, len(udp.Data))
	_, err = client.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v\n", string(b))
}
