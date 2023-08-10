// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

//go:build aix || darwin || (js && wasm) || (solaris && !illumos)
// +build aix darwin js,wasm solaris,!illumos

package netutil

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// Accept wrapper around the accept system call that marks the returned file
// descriptor as close-on-exec.
// Copy from golang source code: internal/poll/sys_cloexec.go
func Accept(fd int) (int, unix.Sockaddr, error) {
	// See ../syscall/exec_unix.go for description of ForkLock.
	// It is probably okay to hold the lock across syscall.Accept
	// because we have put fd.sysfd into non-blocking mode.
	// However, a call to the File method will put it back into
	// blocking mode. We can't take that risk, so no use of ForkLock here.
	ns, sa, err := unix.Accept(fd)
	if err == nil {
		syscall.CloseOnExec(ns)
	}
	if err != nil {
		return -1, nil, err
	}
	return ns, sa, nil
}
