// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package tcp provides common function for tcp example.
package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"log"

	"trpc.group/trpc-go/tnet"
)

var (
	// Address is the listening address
	Address = "127.0.0.1:9990"
	// DataHeaderLen is the length of packet header
	DataHeaderLen = 4
)

// StartServer starts a tcp server.
func StartServer(isIONonBlocking bool, tcpHandler tnet.TCPHandler) {
	// As the default poller number is 1,
	// you possibly would want to set it to a higher value.
	tnet.SetNumPollers(4)
	ln, err := tnet.Listen("tcp", Address)
	if err != nil {
		log.Fatal(err)
	}
	s, err := tnet.NewTCPService(
		ln,
		tcpHandler,
		tnet.WithNonBlocking(isIONonBlocking),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(s.Serve(context.Background()))
}

// BusinessHandle is the business logic for example.
func BusinessHandle(conn tnet.Conn, data []byte) error {
	// Use writev to write header and data in order.
	rspHead := new(bytes.Buffer)
	binary.Write(rspHead, binary.BigEndian, uint32(len(data)))
	if _, err := conn.Writev(rspHead.Bytes(), data); err != nil {
		log.Fatal(err)
	}
	return nil
}
