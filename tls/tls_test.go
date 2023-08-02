// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package tls_test

import (
	"context"
	"crypto/rand"
	stdtls "crypto/tls"
	"errors"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/tls"
)

var (
	port       = "10101"
	listenAddr = ":" + port
	addr       = "127.0.0.1:" + port
)

func TestTLS(t *testing.T) {
	hello := make([]byte, 100)
	rand.Read(hello)
	runTestWithHandles(t, func(c tls.Conn) error {
		c.SetIdleTimeout(time.Second)
		c.SetMetaData(0)
		require.Equal(t, 0, c.GetMetaData().(int))
		buf := make([]byte, len(hello))
		n, err := c.Read(buf)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, hello[:n], buf[:n])
		c.SetFlushWrite(false)
		n, err = c.Write(buf)
		require.Nil(t, err)
		return nil
	}, func(c tls.Conn) error {
		require.True(t, c.IsActive())
		n, err := c.Write(hello)
		require.Nil(t, err)
		buf := make([]byte, len(hello))
		n, err = c.Read(buf)
		require.Nil(t, err)
		require.Equal(t, hello[:n], buf[:n])
		return nil
	}, []tls.ClientOption{
		tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}),
		tls.WithTimeout(time.Second),
		tls.WithClientFlushWrite(true),
		tls.WithClientIdleTimeout(time.Second),
	}, tls.WithServerTLSConfig(getTLSCfg()),
		tls.WithTCPKeepAlive(time.Second),
		tls.WithServerFlushWrite(true),
		tls.WithServerIdleTimeout(time.Second))
}

var testCount = 10000

func TestNormalClientTNetServer(t *testing.T) {
	done := make(chan struct{})
	cancel := runServer(t, func(c tls.Conn) error {
		buf := make([]byte, 5)
		n, err := c.Read(buf)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 5, n)
		n, err = c.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		return nil
	}, done, tls.WithServerTLSConfig(getTLSCfg()))
	conn, err := stdtls.Dial("tcp", addr, &stdtls.Config{InsecureSkipVerify: true})
	require.Nil(t, err)
	buf := make([]byte, 5)
	data := make([]byte, 5)
	for i := 0; i < testCount; i++ {
		rand.Read(buf)
		n, err := conn.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		n, err = conn.Read(data)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, buf, data)
	}
	cancel()
	<-done
}

func TestTNetClientTNetServer(t *testing.T) {
	done := make(chan struct{})
	cancel := runServer(t, func(c tls.Conn) error {
		buf := make([]byte, 5)
		n, err := c.Read(buf)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 5, n)
		n, err = c.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		return nil
	}, done, tls.WithServerTLSConfig(getTLSCfg()))
	conn, err := tls.Dial("tcp", addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	require.Nil(t, err)
	buf := make([]byte, 5)
	data := make([]byte, 5)
	for i := 0; i < testCount; i++ {
		rand.Read(buf)
		n, err := conn.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		n, err = conn.Read(data)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, buf, data)
	}
	cancel()
	<-done
}

func TestTNetClientNormalServer(t *testing.T) {
	cancel := runNormalServer(t)
	defer cancel()
	time.Sleep(time.Second)
	conn, err := tls.Dial("tcp", addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	require.Nil(t, err)
	defer conn.Close()
	buf := make([]byte, 5)
	data := make([]byte, 5)
	for i := 0; i < testCount; i++ {
		rand.Read(buf)
		n, err := conn.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		n, err = conn.Read(data)
		require.Nil(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, buf, data)
	}
}

func TestServer_OnOpenedAndOnClosed(t *testing.T) {
	ch := make(chan struct{}, 1)
	hello := make([]byte, 100)
	rand.Read(hello)
	onOpened := func(conn tls.Conn) error {
		conn.SetMetaData(hello)
		return nil
	}
	onClosed := func(conn tls.Conn) error {
		m := conn.GetMetaData()
		h, ok := m.([]byte)
		require.True(t, ok)
		require.Equal(t, h, hello)
		select {
		case ch <- struct{}{}:
		default:
		}
		return nil
	}
	runTestWithHandles(t, func(c tls.Conn) error {
		data := make([]byte, 100)
		c.Read(data)
		time.Sleep(time.Second)
		return nil
	}, func(c tls.Conn) error {
		_, err := c.Write(hello)
		require.Nil(t, err)
		time.Sleep(time.Millisecond * 100)
		c.Close()
		return nil
	}, []tls.ClientOption{
		tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}),
	}, tls.WithServerTLSConfig(getTLSCfg()),
		tls.WithOnOpened(onOpened),
		tls.WithOnClosed(onClosed),
	)
	waitFor := time.NewTimer(time.Second)
	select {
	case <-ch:
	case <-waitFor.C:
		t.Fatal("tls connection is not closed correctly")
	}
}

func TestClientAndServerForHighConcurrency(t *testing.T) {
	done := make(chan struct{})
	cancel := runServer(t, func(c tls.Conn) error {
		buf := make([]byte, 20)
		n, err := c.Read(buf)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 20, n)
		n, err = c.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 20, n)
		return nil
	}, done, tls.WithServerTLSConfig(getTLSCfg()))
	conn, err := tls.Dial("tcp", addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	require.Nil(t, err)
	buf := make([]byte, 20)
	rand.Read(buf)
	var (
		wg         sync.WaitGroup
		concurrent = 100
		reqNum     = 100
	)
	wg.Add(concurrent * reqNum)
	conn.SetOnRequest(func(c tls.Conn) error {
		data := make([]byte, 20)
		n, err := conn.Read(data)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 20, n)
		require.Equal(t, buf, data)
		wg.Done()
		return nil
	})
	for i := 0; i < concurrent; i++ {
		go func() {
			for i := 0; i < reqNum; i++ {
				n, err := conn.Write(buf)
				require.Nil(t, err)
				require.Equal(t, 20, n)
			}
		}()
	}
	waitFor := time.NewTimer(time.Second)
	wgDone := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		wgDone <- struct{}{}
	}()
	select {
	case <-wgDone:
	case <-waitFor.C:
		t.Fatal("the onRequest of the client is not triggered enough times, some responses are missed")
	}
	cancel()
	<-done
}

func TestClientAndServerForHighConcurrency(t *testing.T) {
	done := make(chan struct{})
	cancel := runServer(t, func(c tls.Conn) error {
		buf := make([]byte, 20)
		n, err := c.Read(buf)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 20, n)
		n, err = c.Write(buf)
		require.Nil(t, err)
		require.Equal(t, 20, n)
		return nil
	}, done, tls.WithServerTLSConfig(getTLSCfg()))
	conn, err := tls.Dial("tcp", addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	require.Nil(t, err)
	buf := make([]byte, 20)
	rand.Read(buf)
	var (
		wg         sync.WaitGroup
		concurrent = 100
		reqNum     = 100
	)
	wg.Add(concurrent * reqNum)
	conn.SetOnRequest(func(c tls.Conn) error {
		data := make([]byte, 20)
		n, err := conn.Read(data)
		if errors.Is(err, tnet.ErrConnClosed) {
			return err
		}
		require.Nil(t, err)
		require.Equal(t, 20, n)
		require.Equal(t, buf, data)
		wg.Done()
		return nil
	})
	for i := 0; i < concurrent; i++ {
		go func() {
			for i := 0; i < reqNum; i++ {
				n, err := conn.Write(buf)
				require.Nil(t, err)
				require.Equal(t, 20, n)
			}
		}()
	}
	require.Eventually(t,
		func() bool {
			wg.Wait()
			return true
		}, time.Second, time.Millisecond*200,
		"The onRequest of the client is not triggered enough times, some responses are missed.")
	cancel()
	<-done
}

func BenchmarkTNetClientServer(b *testing.B) {
	buf := make([]byte, 5)
	ln, err := tnet.Listen("tcp", listenAddr)
	if err != nil {
		b.Fatal("tnet.Listen", err)
	}
	s, err := tls.NewService(ln, func(c tls.Conn) error {
		c.Read(buf)
		c.Write(buf)
		return nil
	}, tls.WithServerTLSConfig(getTLSCfg()))
	if err != nil {
		b.Fatal("tls.NewServer", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Serve(ctx)
		done <- struct{}{}
	}()
	conn, _ := tls.Dial("tcp", addr, tls.WithClientTLSConfig(&stdtls.Config{InsecureSkipVerify: true}))
	data := []byte("hello")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := conn.Write(data)
		if err != nil {
			b.Fatal("conn.Write", err)
		}
		_, err = conn.Read(data)
		if err != nil {
			b.Fatal("conn.Read", err)
		}
	}
	b.StopTimer()
	conn.Close()
	cancel()
	<-done
}

func BenchmarkNormalClientServer(b *testing.B) {
	ln, _ := stdtls.Listen("tcp", listenAddr, getTLSCfg())
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 5)
			go func() {
				defer conn.Close()
				for {
					_, err := conn.Read(buf)
					if err != nil {
						return
					}
					_, err = conn.Write(buf)
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	conn, err := stdtls.Dial("tcp", addr, &stdtls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Println("stdtls.Dial", err)
	}
	data := []byte("hello")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = conn.Write(data)
		if err != nil {
			b.Fatal("conn.Write", err)
		}
		_, err = conn.Read(data)
		if err != nil {
			b.Fatal("conn.Read", err)
		}
	}
	b.StopTimer()
	conn.Close()
	ln.Close()
}

func getTLSCfg() *stdtls.Config {
	cert, err := stdtls.LoadX509KeyPair("testdata/server.crt", "testdata/server.key")
	if err != nil {
		log.Fatalln("load cert", err)
	}
	return &stdtls.Config{Certificates: []stdtls.Certificate{cert}}
}

func runTestWithHandles(
	t *testing.T,
	serverHandle tls.Handler,
	clientHandle tls.Handler,
	dialOpts []tls.ClientOption,
	opts ...tls.ServerOption,
) {
	done := make(chan struct{})
	cancel := runServer(t, serverHandle, done, opts...)
	runTestWithHandlesNormal(t, clientHandle, dialOpts)
	runTestWithHandlesOnRequestOnClose(t, clientHandle, dialOpts)
	cancel()
	<-done
}

func runTestWithHandlesNormal(
	t *testing.T,
	clientHandle tls.Handler,
	dialOpts []tls.ClientOption,
) {
	conn, err := tls.Dial("tcp", addr, dialOpts...)
	require.Nil(t, err)
	require.Nil(t, clientHandle(conn))
}

func runTestWithHandlesOnRequestOnClose(
	t *testing.T,
	clientHandle tls.Handler,
	dialOpts []tls.ClientOption,
) {
	conn, err := tls.Dial("tcp", addr, dialOpts...)
	require.Nil(t, err)
	conn.SetOnRequest(func(_ tls.Conn) error {
		return nil
	})
	conn.SetOnClosed(func(_ tls.Conn) error {
		return nil
	})
	require.Nil(t, clientHandle(conn))
}

func runServer(
	t *testing.T,
	h tls.Handler,
	done chan struct{},
	opts ...tls.ServerOption,
) context.CancelFunc {
	ln, err := tnet.Listen("tcp", listenAddr)
	require.Nil(t, err)
	s, err := tls.NewService(ln, h, opts...)
	require.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.Serve(ctx)
		done <- struct{}{}
	}()
	return cancel
}

func runNormalServer(
	t *testing.T,
) context.CancelFunc {
	ln, err := stdtls.Listen("tcp", listenAddr, getTLSCfg())
	require.Nil(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConn(t, conn)
		}
	}()
	return func() {
		ln.Close()
	}
}

func handleConn(t *testing.T, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 5)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		require.Equal(t, 5, n)
		n, err = conn.Write(buf)
		if err != nil {
			return
		}
		require.Equal(t, 5, n)
	}
}
