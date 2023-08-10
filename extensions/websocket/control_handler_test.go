// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package websocket_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/extensions/websocket"
)

func TestControlHandlers(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Nil(t, conn.WriteMessage(websocket.Binary, buf))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Ping, world))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, hello, data)

		require.Nil(t, conn.WriteMessage(websocket.Pong, world))
		_, data, err = conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, world, data)

		require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		_, data, err = conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, string(hello), string(data))

		return nil
	}, nil, websocket.WithPingHandler(func(c websocket.Conn, b []byte) error {
		require.Equal(t, world, b)
		require.Nil(t, c.WriteMessage(websocket.Binary, hello))
		return nil
	}), websocket.WithPongHandler(func(c websocket.Conn, b []byte) error {
		require.Equal(t, world, b)
		require.Nil(t, c.WriteMessage(websocket.Binary, world))
		return nil
	}))
}

func TestDefaultControlHandlers(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		conn.SetPingHandler(nil)
		conn.SetPongHandler(nil)
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Nil(t, conn.WriteMessage(websocket.Binary, buf))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Ping, world))
		require.Nil(t, conn.WriteMessage(websocket.Pong, world))
		require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, hello, data)
		return nil
	}, nil)
}

func TestNormalClose(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		require.Nil(t, conn.Close())
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		_, _, err := conn.ReadMessage()
		require.NotNil(t, err)
		time.Sleep(time.Millisecond)
		return nil
	}, nil, websocket.WithOnClosed(func(c websocket.Conn) error { return nil }))
}
