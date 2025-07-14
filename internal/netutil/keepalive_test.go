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

package netutil_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

var address = "127.0.0.1:9999"

func TestSetKeepAlive(t *testing.T) {
	ln, err := net.Listen("tcp", address)
	require.Nil(t, err)
	defer ln.Close()
	c := make(chan struct{})
	go func() {
		client, err := net.Dial("tcp", address)
		require.Nil(t, err)
		<-c
		client.Close()
	}()
	conn, err := ln.Accept()
	require.Nil(t, err)
	fd, err := netutil.GetFD(conn)
	require.Nil(t, err)
	err = netutil.SetKeepAlive(fd, 1)
	require.Nil(t, err)
	err = netutil.SetKeepAlive(fd, -1)
	require.NotNil(t, err)
	c <- struct{}{}
}

func TestSetKeepAliveErr(t *testing.T) {
	err := netutil.SetKeepAlive(0, 1)
	require.NotNil(t, err)
}
