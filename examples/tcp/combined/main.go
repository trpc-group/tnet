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
	"encoding/binary"
	"errors"
	"log"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/examples/tcp"
)

// mode 3 (merged IO and business)
// Opened with
//
//  1. tnet.WithNonBlocking(true).
//
//  2. do not use goroutine pool in tcp handler.
//
//     +------↓--------------↑--------+
//     | (read packet) (write packet) |
//     |      ↓              ↑        |
//     |  I/O processing     |        |  poller goroutine
//     |         ↓           |        |
//     |        Business logic        |
//     +------------------------------+
//
// # of goroutines in use = (# of pollers)
//
// Characteristics:
//  1. I/O processing is in poller goroutine, MUST be nonblocking.
//  2. Business logic is in poller goroutine, MUST be nonblocking.
func main() {
	var isIONonBlocking = true
	tcp.StartServer(isIONonBlocking, tcpHandler)
}

func tcpHandler(conn tnet.Conn) error {
	header, err := conn.Peek(tcp.DataHeaderLen)
	if errors.Is(err, tnet.EAGAIN) { // not enough data.
		return nil
	}
	if err != nil {
		log.Fatal(err)
	}
	dataLen := binary.BigEndian.Uint32(header)
	if conn.Len() < tcp.DataHeaderLen+int(dataLen) {
		return err // not enough data, leave the peeked data for next use, the EAGAIN error must be returned.
	}
	conn.Skip(tcp.DataHeaderLen)
	data, err := conn.ReadN(int(dataLen))
	if err != nil {
		log.Fatal(err)
	}
	return tcp.BusinessHandle(conn, data)
}
