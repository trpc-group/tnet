// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package websocket_test

import (
	"context"
	stdtls "crypto/tls"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/extensions/websocket"
)

var (
	wsListenAddr  = ":9980"
	wssListenAddr = ":9443"
	wsURL         = "ws://127.0.0.1:9980"
	wssURL        = "wss://127.0.0.1:9443"
	hello         = []byte("hello")
	world         = []byte("world!")
)

func TestClientHandle(t *testing.T) {
	var conns []websocket.Conn
	done := make(chan struct{})
	cancel := runServer(t, wsListenAddr, func(conn websocket.Conn) error {
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Text, tp)
		require.Equal(t, world, buf)
		return nil
	}, done, websocket.WithHookAfterHandshake(func(ctx context.Context, c websocket.Conn) error {
		conns = append(conns, c)
		return nil
	}))
	clientConn, err := websocket.Dial(wsURL)
	require.Nil(t, err)
	clientHandle := func(conn websocket.Conn) error {
		tp, buf, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		require.Equal(t, websocket.Text, tp)
		return conn.WriteMessage(websocket.Text, buf)
	}
	clientConn.SetOnRequest(clientHandle)
	clientConn.SetOnClosed(func(c websocket.Conn) error { return nil })
	require.True(t, len(conns) != 0)
	// Push data from server connections.
	for i := range conns {
		require.Nil(t, conns[i].WriteMessage(websocket.Text, world))
	}
	require.Nil(t, clientConn.Close())
	cancel()
	<-done // Wait for server closing.
}

func TestServer(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Text, tp)
		require.Nil(t, conn.WriteMessage(websocket.Text, buf))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Text, hello))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, string(hello), string(data))
		return nil
	}, nil)
}

func TestServerBinary(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Nil(t, conn.WriteMessage(websocket.Binary, buf))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, string(hello), string(data))
		return nil
	}, nil)
}

var testMessageNumber = 10000

func TestConcurrentReadWrite(t *testing.T) {
	readMu := sync.Mutex{}
	writeMu := sync.Mutex{}
	runTestWithHandles(t, func(conn websocket.Conn) error {
		for i := 0; i < 200; i++ {
			go func() {
				for {
					readMu.Lock()
					tp, buf, err := conn.ReadMessage()
					readMu.Unlock()
					if err != nil {
						if errors.Is(err, tnet.ErrConnClosed) {
							return
						}
						t.Log("conn.ReadMessage", err)
					}
					require.Nil(t, err)
					require.Equal(t, websocket.Binary, tp)
					writeMu.Lock()
					require.Nil(t, conn.WriteMessage(websocket.Binary, buf))
					writeMu.Unlock()
				}
			}()
		}
		time.Sleep(time.Second)
		return nil
	}, func(conn websocket.Conn) error {
		for i := 0; i < testMessageNumber; i++ {
			require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		}
		for i := 0; i < testMessageNumber; i++ {
			_, data, err := conn.ReadMessage()
			require.Nil(t, err)
			require.Equal(t, hello, data)
		}
		return nil
	}, nil)
}

func TestOptions(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		require.NotNil(t, conn.LocalAddr())
		require.NotNil(t, conn.RemoteAddr())
		require.Nil(t, conn.SetDeadline(time.Now().Add(time.Second)))
		require.Nil(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		require.Nil(t, conn.SetWriteDeadline(time.Now().Add(time.Second)))
		require.Nil(t, conn.SetIdleTimeout(time.Second))
		data, ok := conn.GetMetaData().([]byte)
		require.True(t, ok)
		require.Equal(t, string(hello), string(data))
		tp, buf, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Nil(t, conn.WriteMessage(websocket.Binary, buf))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Binary, world))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, string(world), string(data))
		return nil
	}, nil,
		websocket.WithHookBeforeHandshake(func(ctx context.Context) (context.Context, error) {
			_, ok := websocket.RemoteAddrFromContext(ctx)
			require.True(t, ok)
			_, ok = websocket.UpgraderFromContext(ctx)
			require.True(t, ok)
			return ctx, nil
		}),
		websocket.WithHookAfterHandshake(
			func(ctx context.Context, c websocket.Conn) error {
				_, ok := websocket.LocalAddrFromContext(ctx)
				require.True(t, ok)
				c.SetMetaData(hello)
				return nil
			}),
		websocket.WithNewHandshakeContext(func() context.Context { return context.Background() }),
		websocket.WithIdleTimeout(time.Second),
		websocket.WithOnClosed(func(c websocket.Conn) error { return nil }),
		websocket.WithTCPKeepAlive(time.Second),
	)
}

func TestNextMessageReaderWriter(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		tp, r, err := conn.NextMessageReader()
		require.Nil(t, err)
		require.Equal(t, websocket.Text, tp)
		data, err := io.ReadAll(r)
		require.Nil(t, err)
		require.Equal(t, string(hello), string(data))
		require.Nil(t, conn.WriteMessage(tp, data))
		return nil
	}, func(conn websocket.Conn) error {
		w, err := conn.NextMessageWriter(websocket.Text)
		require.Nil(t, err)
		n, err := w.Write(hello[:2])
		require.Nil(t, err)
		require.Equal(t, 2, n)
		n, err = w.Write(hello[2:])
		require.Nil(t, err)
		require.Equal(t, 3, n)
		require.Nil(t, w.Close())
		tp, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Text, tp)
		require.Equal(t, string(hello), string(data))
		return nil
	}, nil)
}

func TestWritev(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		tp, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Equal(t, append(hello, world...), data)
		require.Nil(t, conn.WritevMessage(websocket.Binary, hello, world))
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WritevMessage(websocket.Binary, hello, world))
		tp, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.Binary, tp)
		require.Equal(t, append(hello, world...), data)
		return nil
	}, nil)
}

func TestSubProtocols(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		require.Equal(t, "superchat", conn.Subprotocol())
		tp, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Nil(t, conn.WriteMessage(tp, data))
		return nil
	}, func(conn websocket.Conn) error {
		require.Equal(t, "superchat", conn.Subprotocol())
		require.Nil(t, conn.WriteMessage(websocket.Text, hello))
		_, data, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, hello, data)
		return nil
	}, []websocket.ClientOption{
		websocket.WithTimeout(time.Second),
		websocket.WithSubProtocols([]string{"chat", "superchat"}),
	}, websocket.WithProtocolSelect(func(b []byte) bool {
		switch s := string(b); s {
		case "chat":
			return true
		default:
			return false
		}
	}), websocket.WithProtocolCustom(func(b []byte) (string, bool) {
		if string(b) == "chat, superchat" {
			return "superchat", true
		}
		return "", true
	}))
}

func TestReadWrite(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		buf := make([]byte, 6)
		n, err := conn.Read(buf)
		if err != nil {
			t.Log("server read", err)
			return err
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			t.Log("server write", err)
			return err
		}
		return nil
	}, func(conn websocket.Conn) error {
		n, err := conn.Write(hello)
		require.Nil(t, err)
		require.Equal(t, len(hello), n)
		buf := make([]byte, len(hello))
		n, err = conn.Read(buf)
		require.Nil(t, err)
		require.Equal(t, len(hello), n)
		require.Equal(t, hello, buf)

		n, err = conn.Write(hello)
		require.Nil(t, err)
		require.Equal(t, len(hello), n)
		n, err = conn.Write(world)
		require.Nil(t, err)
		require.Equal(t, len(world), n)

		buf = make([]byte, len(hello)+len(world))
		n, err = conn.Read(buf[:2])
		require.Nil(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, hello[:2], buf[:2])
		n, err = conn.Read(buf[2:5])
		require.Nil(t, err)
		require.Equal(t, 3, n)
		require.Equal(t, hello[2:5], buf[2:5])
		conn.Close()
		return nil
	}, []websocket.ClientOption{
		websocket.WithClientMessageType(websocket.Binary),
	}, websocket.WithServerMessageType(websocket.Binary))
}

func TestReadWriteErrMessageType(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		buf := make([]byte, 6)
		_, err := conn.Read(buf)
		require.NotNil(t, err)
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Text, hello))
		time.Sleep(time.Millisecond)
		return nil
	}, []websocket.ClientOption{
		websocket.WithClientMessageType(websocket.Binary),
	}, websocket.WithServerMessageType(websocket.Binary))
}

func TestReadWriteWithNoMessageType(t *testing.T) {
	runTestWithHandles(t, func(conn websocket.Conn) error {
		buf := make([]byte, 6)
		n, err := conn.Read(buf)
		require.NotNil(t, err)
		_, err = conn.Write(buf[:n])
		require.NotNil(t, err)
		return nil
	}, func(conn websocket.Conn) error {
		require.Nil(t, conn.WriteMessage(websocket.Binary, hello))
		_, err := conn.Write(hello)
		require.NotNil(t, err)
		buf := make([]byte, len(hello))
		_, err = conn.Read(buf)
		require.NotNil(t, err)
		return nil
	}, nil)
}

func runTestWithHandles(
	t *testing.T,
	serverHandle websocket.Handler,
	clientHandle websocket.Handler,
	dialOpts []websocket.ClientOption,
	opts ...websocket.ServerOption,
) {
	runWSTestWithHandles(t, serverHandle, clientHandle, dialOpts, opts...)
	dialOpts = append(dialOpts, websocket.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	opts = append(opts, websocket.WithServerTLSConfig(getTLSCfg()))
	runWSSTestWithHandles(t, serverHandle, clientHandle, dialOpts, opts...)
}

func getTLSCfg() *stdtls.Config {
	cert, err := stdtls.LoadX509KeyPair("testdata/server.crt", "testdata/server.key")
	if err != nil {
		log.Fatal(err)
	}
	return &stdtls.Config{Certificates: []stdtls.Certificate{cert}}
}

func runWSTestWithHandles(
	t *testing.T,
	serverHandle websocket.Handler,
	clientHandle websocket.Handler,
	dialOpts []websocket.ClientOption,
	opts ...websocket.ServerOption,
) {
	done := make(chan struct{})
	cancel := runServer(t, wsListenAddr, serverHandle, done, opts...)
	conn, err := websocket.Dial(wsURL, dialOpts...)
	require.Nil(t, err)
	require.Nil(t, clientHandle(conn))
	cancel()
	<-done
}

func runWSSTestWithHandles(
	t *testing.T,
	serverHandle websocket.Handler,
	clientHandle websocket.Handler,
	dialOpts []websocket.ClientOption,
	opts ...websocket.ServerOption,
) {
	done := make(chan struct{})
	cancel := runServer(t, wssListenAddr, serverHandle, done, opts...)
	conn, err := websocket.Dial(wssURL, dialOpts...)
	require.Nil(t, err)
	require.Nil(t, clientHandle(conn))
	cancel()
	<-done
}

func runServer(
	t *testing.T,
	addr string,
	h websocket.Handler,
	done chan struct{},
	opts ...websocket.ServerOption,
) context.CancelFunc {
	ln, err := tnet.Listen("tcp", addr)
	if err != nil {
		t.Log("tnet.Listen", err)
	}
	require.Nil(t, err)
	s, err := websocket.NewService(ln, h, opts...)
	require.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.Serve(ctx)
		done <- struct{}{}
	}()
	return cancel
}
