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
	"bytes"
	"encoding/binary"
	"errors"
	"log"

	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/examples/tcp"
)

// mode 2 (separated IO and business)
// Opened with
//
//  1. tnet.WithNonBlocking(true).
//
//  2. specifically use a goroutine pool in tcp handler.
//
//     +------↓--------------↑--------+
//     | (read packet) (write packet) |
//     |      ↓              ↑        |  poller goroutine
//     |  I/O processing     |        |
//     +---------|-----------|--------+
//     +---------↓-----------|--------+
//     |        Business logic        |  goroutine pool
//     +------------------------------+
//
// # of goroutines in use =
//
//	(# of pollers) + concurrency(# of concurrent packet in process)
//
// Characteristics:
//  1. I/O processing is in poller goroutine, MUST be nonblocking.
//  2. Business logic is NOT in poller goroutine, MUST block.
func main() {
	var isIONonBlocking = true
	tcp.StartServer(isIONonBlocking, tcpHandler)
}

func tcpHandler(conn tnet.Conn) error {
	header, err := conn.Peek(tcp.DataHeaderLen)
	if errors.Is(err, tnet.EAGAIN) { // not enough data.
		return err // EAGAIN error must be returned.
	}
	if err != nil {
		log.Println(err)
		return err
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
	tnet.Submit(func() { handle(conn, data) })
	return nil
}

func handle(conn tnet.Conn, data []byte) error {
	// Use writev to write header and data in order.
	rspHead := new(bytes.Buffer)
	binary.Write(rspHead, binary.BigEndian, uint32(len(data)))
	if _, err := conn.Writev(rspHead.Bytes(), data); err != nil {
		log.Fatal(err)
	}
	return nil
}
