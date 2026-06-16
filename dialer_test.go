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
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet"
)

func startNetTCPServer(t *testing.T, network, address string, ch chan string) {
	ln, err := net.Listen(network, address)
	require.Nil(t, err)
	ch <- ln.Addr().String()
	conn, err := ln.Accept()
	require.Nil(t, err)
	for {
		req := make([]byte, 1024)
		n, err := io.ReadAtLeast(conn, req, 1)
		if err != nil {
			return
		}
		m, err := conn.Write(req[:n])
		require.Nil(t, err)
		require.Equal(t, n, m)
	}
}

func startTnetTCPServer(t *testing.T, network, address string, ch chan string) {
	ln, err := tnet.Listen(network, address)
	require.Nil(t, err)
	ch <- ln.Addr().String()

	opts := []tnet.Option{tnet.WithTCPKeepAlive(10 * time.Minute)}
	handle := func(conn tnet.Conn) error {
		req := make([]byte, 1024)
		n, err := io.ReadAtLeast(conn, req, 1)
		if err != nil {
			return err
		}
		m, err := conn.Write(req[:n])
		require.Nil(t, err)
		require.Equal(t, n, m)
		return nil
	}
	s, err := tnet.NewTCPService(ln, handle, opts...)
	require.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Serve(ctx)
}

func TestDialTCP_UnReach(t *testing.T) {
	addr := getTestAddr()
	_, err := tnet.DialTCP("tcp", addr, time.Millisecond*100)
	require.NotNil(t, err)
}

func TestDialTCP_InvalidNetwork(t *testing.T) {
	addr := getTestAddr()
	_, err := tnet.DialTCP("unix", addr, time.Millisecond*100)
	require.NotNil(t, err)
}

func TestDialTCP_Net_Sync(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startNetTCPServer(t, "tcp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialTCP("tcp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()
	for i := 0; i <= 1000; i++ {
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		rsp, err := conn.ReadN(len(helloWorld))
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp)
	}
}

func TestDialTCP_Net_Async(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startNetTCPServer(t, "tcp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialTCP("tcp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()

	wg := sync.WaitGroup{}
	onRequest := func(conn tnet.Conn) error {
		rsp, err := conn.ReadN(len(helloWorld))
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp)
		wg.Done()
		return nil
	}
	assert.Nil(t, conn.SetOnRequest(onRequest))

	for i := 0; i <= 1000; i++ {
		wg.Add(1)
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
	}
	wg.Wait()
}

func TestDialTCP_Tnet_Sync(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startTnetTCPServer(t, "tcp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialTCP("tcp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()
	for i := 0; i <= 1000; i++ {
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		rsp, err := conn.ReadN(len(helloWorld))
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp)
	}
}

func TestDialTCP_Tnet_Async(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startTnetTCPServer(t, "tcp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialTCP("tcp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()

	wg := sync.WaitGroup{}
	onRequest := func(conn tnet.Conn) error {
		rsp, err := conn.ReadN(len(helloWorld))
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp)
		wg.Done()
		return nil
	}
	assert.Nil(t, conn.SetOnRequest(onRequest))

	for i := 0; i <= 10000; i++ {
		wg.Add(1)
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
	}
	wg.Wait()
}

func startNetUDPServer(t *testing.T, network, address string, ch chan string) {
	conn, err := net.ListenPacket(network, address)
	require.Nil(t, err)
	ch <- conn.LocalAddr().String()
	for {
		req := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(req)
		if err != nil {
			fmt.Println("收包错误")
			return
		}
		m, err := conn.WriteTo(req[:n], addr)
		require.Nil(t, err)
		require.Equal(t, n, m)
	}
}

func startTnetUDPServer(t *testing.T, network, address string, ch chan string) {
	lns, err := tnet.ListenPackets(network, address, false)
	require.Nil(t, err)
	s, err := tnet.NewUDPService(lns, func(conn tnet.PacketConn) error {
		req := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(req)
		if err != nil {
			return err
		}
		m, err := conn.WriteTo(req[:n], addr)
		if err != nil {
			fmt.Println("服务端写包失败：", err)
		}
		require.Nil(t, err)
		require.Equal(t, n, m)
		return nil
	})
	require.Nil(t, err)

	ch <- lns[0].LocalAddr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Serve(ctx)
}

func TestDialUDP_InvalidNetwork(t *testing.T) {
	addr := getTestAddr()
	_, err := tnet.DialUDP("tcp", addr, time.Millisecond*100)
	require.NotNil(t, err)
}

func TestDialUDP_Net_Sync(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startNetUDPServer(t, "udp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialUDP("udp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()
	for i := 0; i <= 1000; i++ {
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		rsp := make([]byte, 1024)
		n, err := conn.Read(rsp)
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp[:n])
	}
}

func TestDialUDP_Net_Async(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startNetUDPServer(t, "udp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialUDP("udp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()

	wg := sync.WaitGroup{}
	onRequest := func(conn tnet.PacketConn) error {
		rsp := make([]byte, 1024)
		n, err := conn.Read(rsp)
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp[:n])
		wg.Done()
		return nil
	}
	assert.Nil(t, conn.SetOnRequest(onRequest))

	for i := 0; i <= 100; i++ {
		wg.Add(1)
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		time.Sleep(time.Microsecond)
	}
	wg.Wait()
}

func TestDialUDP_Tnet_Sync(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startTnetUDPServer(t, "udp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialUDP("udp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()

	for i := 0; i <= 1000; i++ {
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		rsp := make([]byte, 1024)
		n, err := conn.Read(rsp)
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp[:n])
	}
}

func TestDialUDP_Tnet_Async(t *testing.T) {
	waitCh := make(chan string)
	addr := getTestAddr()
	go startTnetUDPServer(t, "udp", addr, waitCh)
	addr = <-waitCh
	conn, err := tnet.DialUDP("udp", addr, time.Millisecond*100)
	require.Nil(t, err)
	defer conn.Close()

	wg := sync.WaitGroup{}
	onRequest := func(conn tnet.PacketConn) error {
		rsp := make([]byte, 1024)
		n, err := conn.Read(rsp)
		require.Nil(t, err)
		require.Equal(t, helloWorld, rsp[:n])
		wg.Done()
		return nil
	}
	assert.Nil(t, conn.SetOnRequest(onRequest))

	for i := 0; i <= 100; i++ {
		wg.Add(1)
		_, err = conn.Write(helloWorld)
		require.Nil(t, err)
		time.Sleep(time.Microsecond)
	}
	wg.Wait()
}

// TestDialContextTCP_Success tests successful connection using DialContextTCP
func TestDialContextTCP_Success(t *testing.T) {
	// Start a TCP server
	waitCh := make(chan string)
	addr := getTestAddr()
	go startNetTCPServer(t, "tcp", addr, waitCh)
	serverAddr := <-waitCh

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Connect to the server using DialContextTCP
	conn, err := tnet.DialContextTCP(ctx, "tcp", serverAddr)
	require.Nil(t, err)
	defer conn.Close()

	// Verify connection by sending and receiving data
	_, err = conn.Write(helloWorld)
	require.Nil(t, err)
	rsp, err := conn.ReadN(len(helloWorld))
	require.Nil(t, err)
	assert.Equal(t, helloWorld, rsp)
}

// TestDialContextTCP_CancelledContext tests connection attempt with a cancelled context
func TestDialContextTCP_CancelledContext(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Attempt to connect with cancelled context
	addr := getTestAddr()
	_, err := tnet.DialContextTCP(ctx, "tcp", addr)
	require.NotNil(t, err)
	// The error message might vary, but it should indicate the operation was canceled
	assert.Contains(t, err.Error(), "canceled")
}

// TestDialContextTCP_Timeout tests connection attempt with a context that times out
func TestDialContextTCP_Timeout(t *testing.T) {
	// Create a TCP listener but don't accept any connections
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	defer ln.Close()

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure the context has expired
	time.Sleep(time.Millisecond)

	// Attempt to connect with an expired context
	_, err = tnet.DialContextTCP(ctx, "tcp", ln.Addr().String())
	require.NotNil(t, err)
	// The error message might vary, but it should indicate a timeout or deadline exceeded
	assert.True(t,
		containsAny(err.Error(), []string{"timeout", "deadline exceeded", "i/o timeout", "operation was canceled"}),
		"Error message should indicate timeout: %s", err.Error())
}

// TestDialContextTCP_InvalidNetwork tests connection attempt with an invalid network
func TestDialContextTCP_InvalidNetwork(t *testing.T) {
	ctx := context.Background()
	addr := getTestAddr()
	_, err := tnet.DialContextTCP(ctx, "unix", addr)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "unknown network")
}

// TestDialContextTCP_WithTnetServer tests connection to a tnet TCP server
func TestDialContextTCP_WithTnetServer(t *testing.T) {
	// Start a tnet TCP server
	waitCh := make(chan string)
	addr := getTestAddr()
	go startTnetTCPServer(t, "tcp", addr, waitCh)
	serverAddr := <-waitCh

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Connect to the server using DialContextTCP
	conn, err := tnet.DialContextTCP(ctx, "tcp", serverAddr)
	require.Nil(t, err)
	defer conn.Close()

	// Verify connection by sending and receiving data
	_, err = conn.Write(helloWorld)
	require.Nil(t, err)
	rsp, err := conn.ReadN(len(helloWorld))
	require.Nil(t, err)
	assert.Equal(t, helloWorld, rsp)
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && strings.Contains(s, substr)
}
