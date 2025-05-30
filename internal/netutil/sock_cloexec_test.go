//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

func TestAccept(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	fd, err := netutil.GetFD(ln)
	require.Nil(t, err)

	listenAddr := ln.Addr()
	go func() {
		_, err := net.Dial("tcp", listenAddr.String())
		require.Nil(t, err)
	}()

	time.Sleep(100 * time.Millisecond)
	_, _, err = netutil.Accept(fd)
	assert.Nil(t, err)

	_, _, err = netutil.Accept(10086)
	assert.NotNil(t, err)
}
