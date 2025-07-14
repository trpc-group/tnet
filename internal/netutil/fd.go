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

// Package netutil provides network netutil functions.

//go:build linux || freebsd || dragonfly || darwin
// +build linux freebsd dragonfly darwin

package netutil

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
)

// GetFD returns the integer Unix file descriptor referencing the tcp/udp socket.
func GetFD(socket interface{}) (int, error) {
	conn, ok := socket.(syscall.Conn)
	if !ok {
		return -1, fmt.Errorf("type %T doesn't implement syscall.Conn interface", socket)
	}
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return -1, fmt.Errorf("get raw connection fail %w", err)
	}

	fd := -1
	op := func(sysfd uintptr) {
		fd = int(sysfd)
	}
	err = rawConn.Control(op)
	if fd == -1 {
		return -1, errors.New("invalid file descriptor")
	}
	return fd, err
}

// DupFD duplicates file descriptor and returns the new fd.
func DupFD(socket interface{}) (int, error) {
	var f *os.File
	var err error
	switch conn := socket.(type) {
	case *net.TCPConn:
		f, err = conn.File()
	case *net.UDPConn:
		f, err = conn.File()
	case *net.TCPListener:
		f, err = conn.File()
	default:
		return -1, errors.New("not implement SyscallConn()")
	}
	if err != nil {
		return -1, err
	}
	return int(f.Fd()), nil
}
