// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package tnet_test

import (
	"context"
	"fmt"
	"io"
	"net"
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
	lns, err := tnet.ListenPackets(network, address, true)
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
