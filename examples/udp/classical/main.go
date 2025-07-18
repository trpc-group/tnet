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
	"trpc.group/trpc-go/tnet/examples/udp"
)

// mode 1 (classical Go/net)
// This is the default mode (no option set).
//
//	+------↓--------------↑--------+
//	| (read packet) (write packet) |  poller goroutine
//	+------|--------------↑--------+
//	+------↓--------------|--------+
//	|  I/O processing     |        |
//	|        ↓            |        |  goroutine pool
//	|        Business logic        |
//	+------------------------------+
//
// # of goroutines in use =
//
//	(# of pollers) + (# of active connections)
//
// Characteristics:
//  1. I/O processing is NOT in poller goroutine, MUST block.
//  2. Business logic is NOT in poller goroutine, MUST block.
func main() {
	var (
		isIONonBlocking        = false
		useBusinessRoutinePool = false
	)
	udp.StartServer(isIONonBlocking, useBusinessRoutinePool)
}
