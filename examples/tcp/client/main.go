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
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/examples/tcp"
)

var (
	tcpData        = "hello world!"
	tcpReadTimeOut = time.Second
)

func main() {
	// Create a client to connect to the server.
	client, err := tnet.DialTCP("tcp", tcp.Address, time.Second)
	if err != nil {
		log.Fatal(err)
	}
	// Write header and data.
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(len(tcpData)))
	buf.WriteString(tcpData)
	// Write data to the connection.
	if _, err := client.Write(buf.Bytes()); err != nil {
		log.Fatal(err)
	}

	// Use Zero-Copy APIs to read the header.
	client.SetReadDeadline(time.Now().Add(tcpReadTimeOut))
	header, err := client.Peek(tcp.DataHeaderLen)
	if err != nil {
		log.Fatal(err)
	}
	dataLen := binary.BigEndian.Uint32(header)
	client.Skip(tcp.DataHeaderLen)
	// Read data from the connection.
	data, err := client.ReadN(int(dataLen))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v\n", string(data))
}
