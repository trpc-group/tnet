// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package tnet_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet"
)

func TestNewUDPService_err(t *testing.T) {
	_, err := tnet.NewUDPService([]tnet.PacketConn{}, nil)
	assert.NotNil(t, err)

	lns, err := tnet.ListenPackets("udp", getTestAddr(), true)
	assert.Nil(t, err)
	s, err := tnet.NewUDPService(lns, nil)
	assert.Nil(t, err)
	err = s.Serve(context.Background())
	assert.NotNil(t, err)

	lns2, err := tnet.ListenPackets("udp", getTestAddr(), true)
	assert.Nil(t, err)
	lns = append(lns, lns2...)
	_, err = tnet.NewUDPService(lns, nil)
	assert.NotNil(t, err)

	netConn, err := net.ListenPacket("udp", getTestAddr())
	assert.Nil(t, err)
	ln := &mockPacketConn{c: netConn}
	lns = make([]tnet.PacketConn, 0)
	lns = append(lns, ln)
	_, err = tnet.NewUDPService(lns, nil)
	assert.NotNil(t, err)
}

func TestConvertPacketConn(t *testing.T) {
	netc, err := net.ListenPacket("udp", getTestAddr())
	assert.Nil(t, err)
	tnetc, err := tnet.NewPacketConn(netc)
	assert.Nil(t, err)
	s, err := tnet.NewUDPService([]tnet.PacketConn{tnetc}, handler)
	assert.Nil(t, err)
	go s.Serve(context.Background())
	time.Sleep(100 * time.Millisecond)
	c, err := tnet.DialUDP("udp", tnetc.LocalAddr().String(), 0)
	assert.Nil(t, err)
	_, err = c.Write(helloWorld)
	assert.Nil(t, err)
	p, _, err := c.ReadPacket()
	assert.Nil(t, err)
	data, err := p.Data()
	assert.Nil(t, err)
	assert.Equal(t, helloWorld, data)
}

func handler(pc tnet.PacketConn) error {
	p, addr, err := pc.ReadPacket()
	if err != nil {
		return err
	}
	defer p.Free()
	data, err := p.Data()
	if err != nil {
		return err
	}
	_, err = pc.WriteTo(data, addr)
	if err != nil {
		return err
	}
	return nil
}

type mockPacketConn struct {
	c net.PacketConn
	tnet.PacketConn
}

func (c mockPacketConn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func getTestAddr() string {
	return "127.0.0.1:0"
}

var (
	helloWorld = []byte("helloworld")
)
