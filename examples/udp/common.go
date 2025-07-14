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

// Package udp provides common function for udp example.
package udp

import (
	"context"
	"errors"
	"log"
	"net"

	"trpc.group/trpc-go/tnet"
)

var (
	// Address is the listening address
	Address = "127.0.0.1:9991"
	// Data used to transferred between client and server
	Data = "hello world!"
)

// StartServer starts a udp server.
func StartServer(isIONonBlocking bool, useBusinessRoutinePool bool) {
	// As the default poller number is 1,
	// you possibly would want to set it to a higher value.
	tnet.SetNumPollers(4)
	lns, err := tnet.ListenPackets("udp", Address, false)
	if err != nil {
		log.Fatal(err)
	}
	s, err := tnet.NewUDPService(
		lns,
		newUDPHandler(useBusinessRoutinePool),
		tnet.WithNonBlocking(isIONonBlocking),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(s.Serve(context.Background()))
}

func newUDPHandler(useBusinessRoutinePool bool) func(tnet.PacketConn) error {
	return func(conn tnet.PacketConn) error {
		b := make([]byte, len(Data))
		_, addr, err := conn.ReadFrom(b)
		if errors.Is(err, tnet.EAGAIN) { // not enough data.
			return nil
		}
		if err != nil {
			log.Fatal(err)
		}
		if useBusinessRoutinePool {
			tnet.Submit(func() { handle(conn, b, addr) })
		} else {
			handle(conn, b, addr)
		}
		return nil
	}
}

func handle(conn tnet.PacketConn, data []byte, addr net.Addr) error {
	if _, err := conn.WriteTo(data, addr); err != nil {
		return err
	}
	return nil
}
