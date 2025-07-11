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

package tnet_test

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/internal/buffer"
)

var (
	hello          = []byte("hello")
	world          = []byte("world")
	dialRetryTimes = 25
)

type testCase struct {
	servHandle    func(t *testing.T, conn tnet.Conn, ch chan int) error
	clientHandle  func(t *testing.T, conn net.Conn, ch chan int)
	ctrlHandle    func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int)
	name          string
	isTnetCliConn bool
}

func doTestCase(t *testing.T, tt testCase, serverOpts ...tnet.Option) {
	doTestCaseWithOptions(t, tt, serverOpts...)
	doTestCaseWithOptions(t, tt, append(serverOpts, tnet.WithSafeWrite(true))...)
}

func doTestCaseWithOptions(t *testing.T, tt testCase, serverOpts ...tnet.Option) {
	var (
		serverConn  tnet.Conn
		clientConn  net.Conn
		waitChannel = make(chan int)
	)
	// Set up server.
	ln, err := tnet.Listen("tcp", getTestAddr())
	require.Nil(t, err)
	ch := make(chan struct{}, 1)
	serverOpts = append(serverOpts, tnet.WithOnTCPOpened(func(conn tnet.Conn) error {
		serverConn = conn.(tnet.Conn)
		ch <- struct{}{}
		return nil
	}))
	s, err := tnet.NewTCPService(ln, func(conn tnet.Conn) error {
		return tt.servHandle(t, serverConn, waitChannel)
	}, serverOpts...)
	require.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		t.Log("cancel server")
		cancel()
	}()
	go func() {
		t.Log("server serve ", s.Serve(ctx))
	}()

	// Set up client.
	for i := 0; i < dialRetryTimes; i++ {
		time.Sleep(35 * time.Millisecond)
		if tt.isTnetCliConn {
			clientConn, err = tnet.DialTCP(ln.Addr().Network(), ln.Addr().String(), 5*time.Second)
		} else {
			clientConn, err = net.DialTimeout(ln.Addr().Network(), ln.Addr().String(), 5*time.Second)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("isTnetCliConn: %v, dial error: %v", tt.isTnetCliConn, err)
	}
	require.Nil(t, err)
	defer clientConn.Close()
	// Run the client handler.
	if tt.clientHandle != nil {
		tt.clientHandle(t, clientConn, waitChannel)
	}
	<-ch
	// Run the control handler.
	if tt.ctrlHandle != nil {
		tt.ctrlHandle(t, serverConn, clientConn, waitChannel)
	}
}

func TestConnClose_ClientClose(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by client",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
			conn.Close()
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond * 5)
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_IOClose(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by request routine",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			conn.Close()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_APIClose(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by business routine",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			assert.Nil(t, server.Close())
			// Close twice.
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_ReadNBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by server with readN() blocked",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			_, err := conn.ReadN(2 * len(helloWorld))
			assert.NotNil(t, err)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			// Make sure ReadN is blocked.
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_PeekBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by server with Peek() blocked",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			_, err := conn.Peek(2 * len(helloWorld))
			assert.NotNil(t, err)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			// Make sure ReadN is blocked.
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
	})
}

func TestServerConnClose_PeekBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by server write finished, and client peek tcp buffer",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			assert.Nil(t, conn.Close())
			// write finished and close the connection, notify client to peek the data
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			tConn, ok := conn.(tnet.Conn)
			assert.True(t, ok)

			// write a request to trigger the server to write data
			_, err := tConn.Write(helloWorld)
			assert.Nil(t, err)

			// after server write finished and close the connection, peek the data
			<-ch
			// make sure the client has closed the connection
			time.Sleep(50 * time.Millisecond)

			n, err := tConn.Peek(len(helloWorld))
			assert.Nil(t, err)
			assert.Equal(t, helloWorld, n)
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
		isTnetCliConn: true,
	})
}

func TestClientConnClose_PeekBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by client write finished, and server peek tcp buffer",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			// make sure the client has written data and closed the connection
			time.Sleep(50 * time.Millisecond)
			buf, err := conn.Peek(len(helloWorld))
			assert.Nil(t, err)
			assert.Equal(t, helloWorld, buf)
			assert.Nil(t, conn.Close())
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			tConn, ok := conn.(tnet.Conn)
			assert.True(t, ok)

			// write a request to trigger the server to write data
			n, err := tConn.Write(helloWorld)
			assert.Nil(t, err)
			assert.Equal(t, len(helloWorld), n)

			assert.NoError(t, tConn.Close())
			// write finished and close the connection, notify server to peek the data
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
		isTnetCliConn: true,
	})
}

func TestConnClose_SkipBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by server with Skip() blocked",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			err := conn.Skip(2 * len(helloWorld))
			assert.NotNil(t, err)
			conn.Release()
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_ReadBlocked(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by server with Read() blocked",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			req := make([]byte, 2*len(helloWorld))
			_, err := io.ReadFull(conn, req)
			assert.NotNil(t, err)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.Nil(t, server.Close())
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_WriteError(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by write error",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			conn.Close()
			_, err := conn.Write(helloWorld)
			assert.NotNil(t, err)
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnClose_ConcurrentClose(t *testing.T) {
	doTestCase(t, testCase{
		name: "close by write error",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			wg := sync.WaitGroup{}
			for i := 0; i <= 10; i++ {
				wg.Add(1)
				go func() {
					assert.Nil(t, conn.Close())
					wg.Done()
				}()
			}
			wg.Wait()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.False(t, server.IsActive())
		},
	})
}

func TestConnWrite_ServHandleErr(t *testing.T) {
	doTestCase(t, testCase{
		name: "server close connection when handle error",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			return errors.New("handle error")
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond)
			assert.False(t, server.IsActive())
		},
		isTnetCliConn: true,
	})
}

var testTCPPKGNum = 10000

func clientWriteAndReadData(t *testing.T, conn net.Conn, ch chan int) {
	for i := 0; i < testTCPPKGNum; i++ {
		_, err := conn.Write(helloWorld)
		require.Nil(t, err)
	}
	for i := 0; i < testTCPPKGNum; i++ {
		_, err := conn.Write(hello)
		require.Nil(t, err)
		_, err = conn.Write(world)
		require.Nil(t, err)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	go func() {
		rsp := make([]byte, len(helloWorld))
		for i := 0; i < testTCPPKGNum*2; i++ {
			n, err := io.ReadFull(conn, rsp)
			require.Nil(t, err)
			require.Equal(t, len(helloWorld), n)
			assert.Equal(t, helloWorld, rsp)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestConnRead_ReadN(t *testing.T) {
	doTestCase(t, testCase{
		name: "call ReadN",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.ReadN(len(helloWorld))
			require.Nil(t, err)
			_, err = conn.Write(req)
			require.Nil(t, err)
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnRead_BufferFull(t *testing.T) {
	n := (buffer.MaxBufferSize+6*1024)/len(helloWorld) + 1
	doTestCase(t, testCase{
		name: "read buffer full",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			conn.SetOnClosed(func(tnet.Conn) error {
				ch <- 1
				return nil
			})
			for {
				conn.ReadN(buffer.MaxBufferSize)
				return errors.New("buffer full")
			}
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i <= n; i++ {
				_, err := conn.Write(helloWorld)
				assert.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			<-ch
			assert.False(t, server.IsActive())
		},
		isTnetCliConn: true,
	})
}

func TestConnRead_Read(t *testing.T) {
	doTestCase(t, testCase{
		name: "call Read",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req := make([]byte, len(helloWorld))
			n, err := io.ReadFull(conn, req)
			require.Nil(t, err)
			require.Equal(t, n, len(helloWorld))
			require.Equal(t, req, helloWorld)
			_, err = conn.Write(req)
			require.Nil(t, err)
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnRead_PeekAndSkip(t *testing.T) {
	doTestCase(t, testCase{
		name: "call peek and skip",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.Peek(len(helloWorld))
			assert.Nil(t, err)
			assert.Equal(t, helloWorld, req)
			rsp := make([]byte, len(helloWorld))
			copy(rsp, req)
			_, err = conn.Write(rsp)
			assert.Nil(t, err)
			err = conn.Skip(len(helloWorld))
			assert.Nil(t, err)
			conn.Release()
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnRead_Next(t *testing.T) {
	doTestCase(t, testCase{
		name: "call next",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.Next(len(helloWorld))
			assert.Nil(t, err)
			assert.Equal(t, helloWorld, req)
			rsp := make([]byte, len(helloWorld))
			copy(rsp, req)
			_, err = conn.Write(rsp)
			assert.Nil(t, err)
			conn.Release()
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnRead_ConcurrentReadN(t *testing.T) {
	doTestCase(t, testCase{
		name: "concurrent call peek and skip",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			readFunc := func() {
				for {
					req, err := conn.ReadN(len(helloWorld))
					if err != nil {
						return
					}
					_, err = conn.Write(req)
					assert.Nil(t, err)
				}
			}
			for i := 0; i < 10; i++ {
				go readFunc()
			}
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnWrite_AsyncWrite(t *testing.T) {
	doTestCase(t, testCase{
		name: "concurrent call peek and skip",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.ReadN(len(helloWorld))
			require.Nil(t, err)
			require.Equal(t, helloWorld, req)
			go func() {
				_, err := conn.Writev(hello, world)
				require.Nil(t, err)
			}()
			return nil
		},
		clientHandle: clientWriteAndReadData})
}

func TestConnWrite_AsyncWrite_Flush(t *testing.T) {
	doTestCase(t, testCase{
		name: "concurrent call peek and skip",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.ReadN(len(helloWorld))
			require.Nil(t, err)
			require.Equal(t, helloWorld, req)
			go func() {
				_, err := conn.Writev(hello, world)
				require.Nil(t, err)
			}()
			return nil
		},
		clientHandle: clientWriteAndReadData},
		tnet.WithTCPFlushWrite(true))
}

func TestConnReadTimeout(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			err = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConnReadTimeout_Zero(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Time{})
			assert.Nil(t, err)
		},
	}
	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConnReadTimeout_notActive(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Close()
			err := conn.SetReadDeadline(time.Time{})
			assert.Equal(t, tnet.ErrConnClosed, err)
		},
	}
	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConnReadTimeout_reset(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)

			err = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConnReadTimeout_clean(t *testing.T) {
	var count int
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			if count != 0 {
				time.Sleep(time.Hour)
			}
			req := make([]byte, len(helloWorld))
			n, err := conn.Read(req)
			assert.Nil(t, err)
			n, err = conn.Write(req[:n])
			assert.Equal(t, len(helloWorld), n)
			assert.Nil(t, err)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
			assert.Nil(t, err)

			_, err = conn.Write(helloWorld)
			assert.Nil(t, err)

			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.Nil(t, err)

			err = conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
			assert.Nil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConn_SetDeadline(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetDeadline(time.Now().Add(5 * time.Millisecond))
			assert.Nil(t, err)

			time.Sleep(5 * time.Millisecond)

			_, err = conn.Write(helloWorld)
			assert.NotNil(t, err)

			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConn_SetWriteDeadline(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetWriteDeadline(time.Now().Add(time.Millisecond))
			assert.Nil(t, err)

			time.Sleep(6 * time.Millisecond)

			_, err = conn.Write(helloWorld)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConn_SetDeadline_err(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Close()
			err := conn.SetDeadline(time.Now().Add(50 * time.Millisecond))
			assert.NotNil(t, err)
			err = conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
			assert.NotNil(t, err)
		},
	}

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConn_SetKeepAlive(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetDeadline(time.Now().Add(2 * time.Millisecond))
			assert.Nil(t, err)

			time.Sleep(2 * time.Millisecond)

			_, err = conn.Write(helloWorld)
			assert.NotNil(t, err)

			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "keepAlive sets to default value"
	tt.isTnetCliConn = true
	doTestCase(t, tt)

	keepAlive := tnet.WithTCPKeepAlive(time.Millisecond)
	tt.name = "keepAlive round up to 1 second"
	tt.isTnetCliConn = false
	doTestCase(t, tt, keepAlive)

	keepAlive = tnet.WithTCPKeepAlive(0)
	tt.name = "keepAlive turned off"
	tt.isTnetCliConn = true
	doTestCase(t, tt, keepAlive)
}

func TestConn_SetKeepAlive_err(t *testing.T) {
	tt := testCase{
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			ch <- 1
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
			conn.Close()
		},
		ctrlHandle: func(t *testing.T, server tnet.Conn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond * 5)
			assert.False(t, server.IsActive())
			err := server.SetKeepAlive(0)
			assert.NotNil(t, err)
		},
	}

	tt.name = "set keepAlive when conn closed"
	tt.isTnetCliConn = true
	doTestCase(t, tt)
}

func TestConn_SetReadIdleTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	tcp, err := tnet.DialTCP("tcp", ln.Addr().String(), 1*time.Second)
	assert.Nil(t, err)
	err = tcp.Close()
	assert.Nil(t, err)
	err = tcp.SetReadIdleTimeout(1 * time.Second)
	assert.Equal(t, err, tnet.ErrConnClosed)
	svrHandle := func(t *testing.T, conn tnet.Conn, ch chan int) error {
		_, err := conn.ReadN(20)
		assert.Equal(t, err, tnet.ErrConnClosed)
		ch <- 1
		return nil
	}
	tt := testCase{
		servHandle:    svrHandle,
		isTnetCliConn: true,
		name:          "ReadIdleTimeout 1 second close client",
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write([]byte("x"))
			assert.Nil(t, err)
			<-ch
		},
	}
	readIdleTimeout := tnet.WithTCPReadIdleTimeout(1 * time.Second)
	doTestCase(t, tt, readIdleTimeout)
}

func TestConn_SetWriteIdleTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	tcp, err := tnet.DialTCP("tcp", ln.Addr().String(), 1*time.Second)
	assert.Nil(t, err)
	err = tcp.Close()
	assert.Nil(t, err)
	err = tcp.SetWriteIdleTimeout(1 * time.Second)
	assert.Equal(t, err, tnet.ErrConnClosed)
	ln, err = net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	tcp, err = tnet.DialTCP("tcp", ln.Addr().String(), 1*time.Second)
	assert.Nil(t, err)
	err = tcp.SetWriteIdleTimeout(1 * time.Second)
	assert.Nil(t, err)
	err = tcp.SetWriteIdleTimeout(1 * time.Second)
	assert.Nil(t, err)
	err = tcp.SetReadIdleTimeout(1 * time.Second)
	assert.Nil(t, err)
	err = tcp.SetReadIdleTimeout(1 * time.Second)
	assert.Nil(t, err)
	err = tcp.Close()
	assert.Nil(t, err)
	svrHandle := func(t *testing.T, conn tnet.Conn, ch chan int) error {
		str, err := conn.ReadN(1)
		assert.Nil(t, err)
		assert.Equal(t, "x", string(str))
		time.Sleep(1 * time.Second)
		_, err = conn.Write([]byte("helloworld"))
		assert.Nil(t, err)
		_, err = conn.ReadN(20)
		assert.Equal(t, err, tnet.ErrConnClosed)
		ch <- 1
		return nil
	}
	tt := testCase{
		servHandle:    svrHandle,
		isTnetCliConn: true,
		name:          "WriteIdleTimeout 3 seconds close client",
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write([]byte("x"))
			assert.Nil(t, err)
			buf := make([]byte, 10)
			_, err = conn.Read(buf)
			assert.Nil(t, err)
			<-ch
		},
	}
	writeIdleTimeout := tnet.WithTCPWriteIdleTimeout(2 * time.Second)
	doTestCase(t, tt, writeIdleTimeout)
}

func TestConn_SetIdleTimeout(t *testing.T) {
	svrHandle := func(t *testing.T, conn tnet.Conn, ch chan int) error {
		_, err := conn.ReadN(20)
		assert.Equal(t, err, tnet.ErrConnClosed)
		ch <- 1
		return nil
	}
	tt := testCase{
		servHandle:    svrHandle,
		isTnetCliConn: true,
		name:          "idle timeout 1 second close client",
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write([]byte("x"))
			assert.Nil(t, err)
			<-ch
		},
	}
	idleTimeout := tnet.WithTCPIdleTimeout(time.Second)
	doTestCase(t, tt, idleTimeout)
}

func TestTCPConnMetaData(t *testing.T) {
	ln, err := tnet.Listen("tcp", getTestAddr())
	require.Nil(t, err)

	c, err := tnet.DialTCP(ln.Addr().Network(), ln.Addr().String(), time.Second)
	assert.Nil(t, err)

	tc, ok := c.(tnet.Conn)
	assert.True(t, ok)

	tc.SetMetaData(helloWorld)
	ctx := tc.GetMetaData()

	b, ok := ctx.([]byte)
	assert.True(t, ok)
	assert.Equal(t, helloWorld, b)
}

func TestConnRead_NonBlocking(t *testing.T) {
	doTestCase(t, testCase{
		name: "call nonblocking read",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			req, err := conn.ReadN(len(helloWorld))
			if err != nil {
				return err
			}
			_, err = conn.Write(req)
			require.Nil(t, err)
			return nil
		},
		clientHandle: clientWriteAndReadData},
		tnet.WithNonBlocking(true),
	)
}

func TestConn_OnClose(t *testing.T) {
	onClosed := func(conn tnet.Conn) error {
		// After conn.Close(), we can read the data from the connection buffer,
		// but we can't write data to the connection
		buf, err := conn.Next(1)
		assert.NoError(t, err)
		assert.Equal(t, buf[0], helloWorld[0])

		_, err = conn.Write(helloWorld)
		assert.Error(t, err)
		data := conn.GetMetaData()
		assert.Equal(t, helloWorld, data.([]byte))
		return nil
	}
	doTestCase(t, testCase{
		name: "call nonblocking read",
		servHandle: func(t *testing.T, conn tnet.Conn, ch chan int) error {
			conn.SetMetaData(helloWorld)
			conn.Close()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Write(helloWorld)
			<-ch
		},
	},
		tnet.WithOnTCPClosed(onClosed),
	)
}

func TestMassiveConnections(t *testing.T) {
	oldVal := tnet.DefaultCleanUpThrottle
	tnet.DefaultCleanUpThrottle = 0
	defer func() {
		tnet.DefaultCleanUpThrottle = oldVal
	}()
	tnet.MassiveConnections.Store(true)
	massiveConnCnt, packetsPerConn := 5, 10
	ln, err := tnet.Listen("tcp", getTestAddr())
	require.Nil(t, err)
	s, err := tnet.NewTCPService(ln, func(conn tnet.Conn) error {
		buf, err := conn.ReadN(len(hello))
		require.Nil(t, err)
		_, err = conn.Write(buf)
		require.Nil(t, err)
		return err
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx)
	time.Sleep(50 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < massiveConnCnt; i++ {
		wg.Add(1)
		time.Sleep(time.Millisecond * time.Duration(rand.Int()%10))
		go func() {
			defer wg.Done()
			conn, err := tnet.DialTCP("tcp", ln.Addr().String(), time.Second)
			for j := 0; j < packetsPerConn; j++ {
				require.Nil(t, err)
				_, err = conn.Write(hello)
				require.Nil(t, err)
				_, err = conn.ReadN(len(hello))
				require.Nil(t, err)
			}
		}()
	}
	wg.Wait()
}

func TestResetIdleTimeout(t *testing.T) {
	ln, err := tnet.Listen("tcp", getTestAddr())
	require.Nil(t, err)
	s, err := tnet.NewTCPService(ln, func(conn tnet.Conn) error {
		buf, err := conn.ReadN(len(hello))
		require.Nil(t, err)
		_, err = conn.Write(buf)
		require.Nil(t, err)
		return err
	})
	require.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx)
	time.Sleep(50 * time.Millisecond)

	conn, err := tnet.DialTCP("tcp", ln.Addr().String(), time.Second)
	require.Nil(t, err)
	idleTimeout := time.Second
	conn.SetIdleTimeout(idleTimeout)
	time.Sleep(idleTimeout * 2)
	require.False(t, conn.IsActive())

	conn, err = tnet.DialTCP("tcp", ln.Addr().String(), time.Second)
	require.Nil(t, err)
	conn.SetIdleTimeout(idleTimeout)
	conn.SetIdleTimeout(-1) // Clear idle timeout.
	time.Sleep(idleTimeout * 2)
	require.True(t, conn.IsActive())
}
