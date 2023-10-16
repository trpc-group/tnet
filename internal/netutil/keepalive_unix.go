// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build linux || freebsd || dragonfly
// +build linux freebsd dragonfly

package netutil

import "golang.org/x/sys/unix"

// SetKeepAlive turns on keep-alive option for fd and sets the keep-alive interval.
func SetKeepAlive(fd, secs int) error {
	// Turn on keep-alive.
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
		return err
	}
	// Set keep-alive interval.
	// Option TCP_KEEPINTVL controls the time (in seconds) between individual keepalive probes.
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, secs); err != nil {
		return err
	}
	// Set keep-alive idle.
	// Option TCP_KEEPIDLE controls The time (in seconds) the connection needs to remain idle
	// before TCP starts sending keepalive probes, if the socket option SO_KEEPALIVE has been
	// set on this socket.
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, secs)
}
