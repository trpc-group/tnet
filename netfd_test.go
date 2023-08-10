// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package tnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/netutil"
)

var helloWorld = []byte("helloWorld")

type udpClient struct {
	conn    net.PacketConn
	network string
}

func newUDPClient(network string) (*udpClient, error) {
	conn, err := rawListenUDP(network)
	if err != nil {
		return nil, err
	}
	return &udpClient{network: network, conn: conn}, nil
}

func (c *udpClient) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *udpClient) WriteTo(p []byte, addr string) error {
	dst, err := net.ResolveUDPAddr(c.network, addr)
	if err != nil {
		return err
	}
	_, err = c.conn.WriteTo(p, dst)
	if err != nil {
		return err
	}
	return nil
}

func (c *udpClient) ReadFrom() ([]byte, net.Addr, error) {
	p := make([]byte, 20)
	n, addr, err := c.conn.ReadFrom(p)
	if err != nil {
		return nil, nil, err
	}
	p = p[:n]
	return p, addr, nil
}

func (c *udpClient) Close() {
	c.conn.Close()
}

func rawListenUDP(network string) (net.PacketConn, error) {
	var conn net.PacketConn
	var err error
	if network == "udp4" {
		conn, err = net.ListenPacket("udp4", "127.0.0.1:0")
	} else if network == "udp6" {
		conn, err = net.ListenPacket("udp6", "[::1]:0")
	}
	if err != nil {
		return nil, err
	}
	return conn, err
}

func rawToNetFD(rawConn net.PacketConn) (*netFD, error) {
	fd, err := netutil.GetFD(rawConn)
	if err != nil {
		rawConn.Close()
		return nil, err
	}
	nfd := &netFD{
		fd:            fd,
		fdtype:        fdUDP,
		sock:          rawConn,
		network:       "udp",
		laddr:         rawConn.LocalAddr(),
		udpBufferSize: defaultUDPBufferSize,
	}
	return nfd, nil
}

func Test_netFD_FillToBuffer(t *testing.T) {
	rawConn, err := rawListenUDP("udp4")
	assert.Nil(t, err)
	defer rawConn.Close()
	serverAddr := rawConn.LocalAddr()
	nfd, err := rawToNetFD(rawConn)
	assert.Nil(t, err)

	p := []byte{3, 2, 1}
	client, err := newUDPClient("udp4")
	assert.Nil(t, err)
	defer rawConn.Close()
	defer client.Close()
	clientAddr := client.LocalAddr()
	client.WriteTo(p, serverAddr.String())

	b := buffer.New()
	err = nfd.FillToBuffer(b)
	assert.Nil(t, err)

	block, _ := b.ReadBlock()
	s, addr, err := getUDPDataAndAddr(block)
	assert.Nil(t, err)
	assert.Equal(t, p, s)
	assert.Equal(t, clientAddr, addr)
}

func Test_netFD_WriteToIPv4(t *testing.T) {
	var serverAddr net.Addr
	var clientAddr net.Addr
	wait := make(chan struct{}, 1)
	go func() {
		client, err := newUDPClient("udp4")
		assert.Nil(t, err)
		wait <- struct{}{}
		defer client.Close()
		clientAddr = client.LocalAddr()
		s, addr, err := client.ReadFrom()
		assert.Nil(t, err)
		assert.Equal(t, helloWorld, s)
		assert.Equal(t, addr, serverAddr)
		wait <- struct{}{}
	}()
	<-wait
	rawConn, err := rawListenUDP("udp4")
	assert.Nil(t, err)
	defer rawConn.Close()
	serverAddr = rawConn.LocalAddr()
	nfd, err := rawToNetFD(rawConn)
	assert.Nil(t, err)
	n, err := nfd.WriteTo(helloWorld, clientAddr)
	assert.Nil(t, err)
	assert.Equal(t, len(helloWorld), n)
	<-wait
}

func Test_netFD_WriteTo_err(t *testing.T) {
	rawConn, err := rawListenUDP("udp4")
	assert.Nil(t, err)
	defer rawConn.Close()
	nfd, err := rawToNetFD(rawConn)
	assert.Nil(t, err)

	_, err = nfd.WriteTo(helloWorld, nil)
	assert.NotNil(t, err)

	clientAddr := "127.0.0.1:0"
	udpAddr, _ := net.ResolveUDPAddr("udp", clientAddr)
	p := make([]byte, defaultUDPBufferSize+1)
	_, err = nfd.WriteTo(p, udpAddr)
	assert.NotNil(t, err)
}

func TestSetNoDelay_Error(t *testing.T) {
	netFD := &netFD{}
	assert.NotNil(t, netFD.SetNoDelay(false))
}
