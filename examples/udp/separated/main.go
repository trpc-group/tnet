// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package main is the main package.
package main

import (
	"trpc.group/trpc-go/tnet/examples/udp"
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
	var (
		isIONonblocking        = true
		useBusinessRoutinePool = true
	)
	udp.StartServer(isIONonblocking, useBusinessRoutinePool)
}
