// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build freebsd || dragonfly || darwin
// +build freebsd dragonfly darwin

package poller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/iovec"
)

func TestNormal(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	readStream, writeStream := fds[0], fds[1]
	require.Nil(t, err)
	unix.SetNonblock(readStream, true)
	unix.SetNonblock(writeStream, true)
	pollDesc := alloc()
	pollDesc.FD = readStream
	pollDesc.Data = 0
	var onRead, onWrite int
	pollDesc.OnRead = func(interface{}, *iovec.IOData) error {
		onRead++
		buf := make([]byte, 16)
		n, err := unix.Read(pollDesc.FD, buf)
		assert.Nil(t, err)
		assert.Equal(t, 10, n)
		return nil
	}
	pollDesc.OnWrite = func(interface{}) error {
		onWrite++
		return nil
	}
	pollDesc.PickPoller()
	pollDesc.Control(Readable)
	n, err := unix.Write(writeStream, []byte("helloworld"))
	require.Nil(t, err)
	assert.Equal(t, n, 10)
	time.Sleep(time.Second)

	pollDesc.Control(ModWritable)
	n, err = unix.Write(writeStream, []byte("helloworld"))
	require.Nil(t, err)
	assert.Equal(t, n, 10)

	time.Sleep(time.Second)
	pollDesc.Control(Detach)
	assert.Equal(t, 1, onRead)
	assert.Equal(t, 0, onWrite)
}
