// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build dragonfly || freebsd || illumos || linux || netbsd || openbsd
// +build dragonfly freebsd illumos linux netbsd openbsd

package netutil

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// Accept wrapper around the accept system call that marks the returned file
// descriptor as close-on-exec.
// Copy from golang source code: internal/poll/sock_cloexec.go
func Accept(fd int) (int, unix.Sockaddr, error) {
	ns, sa, err := unix.Accept4(fd, syscall.SOCK_CLOEXEC|syscall.SOCK_NONBLOCK)
	// On Linux the accept4 system call was introduced in 2.6.28
	// kernel and on FreeBSD it was introduced in 10 kernel. If we
	// get an ENOSYS error on both Linux and FreeBSD, or EINVAL
	// error on Linux, fall back to using accept.
	switch err {
	case nil:
		return ns, sa, nil
	default: // errors other than the ones listed
		return -1, sa, err
	case syscall.ENOSYS: // syscall missing
	case syscall.EINVAL: // some Linux use this instead of ENOSYS
	case syscall.EACCES: // some Linux use this instead of ENOSYS
	case syscall.EFAULT: // some Linux use this instead of ENOSYS
	}

	// See ../syscall/exec_unix.go for description of ForkLock.
	// It is probably okay to hold the lock across syscall.Accept
	// because we have put fd.sysfd into non-blocking mode.
	// However, a call to the File method will put it back into
	// blocking mode. We can't take that risk, so no use of ForkLock here.
	ns, sa, err = unix.Accept(fd)
	if err == nil {
		syscall.CloseOnExec(ns)
		syscall.SetNonblock(ns, true)
	}
	if err != nil {
		return -1, nil, err
	}
	return ns, sa, nil
}
