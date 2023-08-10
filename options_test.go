// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package tnet

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTNETOptions(t *testing.T) {
	opts := &options{}

	WithTCPKeepAlive(time.Second * 2).f(opts)
	assert.Equal(t, opts.tcpKeepAlive, time.Second*2)

	WithNonBlocking(true).f(opts)
	assert.Equal(t, opts.nonblocking, true)

	handler := func(conn Conn) error {
		return errors.New("test")
	}
	WithOnTCPOpened(handler).f(opts)
	assert.Equal(t, opts.onTCPOpened(nil), handler(nil))

	assert.Nil(t, SetNumPollers(4))
	assert.Equal(t, 4, NumPollers())

	WithMaxUDPPacketSize(1024).f(opts)
	assert.Equal(t, opts.maxUDPPacketSize, 1024)

	WithOnTCPClosed(onTCPClosed).f(opts)
	assert.NotNil(t, opts.onTCPClosed)

	WithOnUDPClosed(onUDPClosed).f(opts)
	assert.NotNil(t, opts.onUDPClosed)

	WithFlushWrite(true).f(opts)

	WithSafeWrite(true).f(opts)
	assert.Equal(t, true, opts.safeWrite)
}

func onTCPClosed(conn Conn) error {
	return nil
}

func onUDPClosed(conn PacketConn) error {
	return nil
}
