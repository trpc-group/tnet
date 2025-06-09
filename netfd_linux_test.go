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

//go:build linux
// +build linux

package tnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/buffer"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
)

func Test_netFD_FillToBuffer(t *testing.T) {
	// Use fixed udp buffer size, 65536 in default.
	t.Run("FixedUDPBufferSize", func(t *testing.T) {
		rawConn, err := rawListenUDP("udp4")
		assert.Nil(t, err)
		defer rawConn.Close()
		serverAddr := rawConn.LocalAddr()
		nfd, err := rawToNetFD(rawConn)
		assert.Nil(t, err)

		p := []byte{3, 2, 1}
		client, err := newUDPClient("udp4")
		assert.Nil(t, err)
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
	})
	// Use exact UDP buffer size based on the packet.
	t.Run("ExactUDPBufferSize", func(t *testing.T) {
		lns, err := listenUDP("udp", "127.0.0.1:", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(lns))
		ln := lns[0]
		defer ln.Close()
		ln.SetExactUDPBufferSizeEnabled(true)
		serverAddr := ln.LocalAddr()
		udpConn, ok := ln.(*udpconn)
		assert.True(t, ok)

		p := []byte{6, 5, 4}
		client, err := newUDPClient("udp4")
		assert.Nil(t, err)
		defer client.Close()
		clientAddr := client.LocalAddr()
		client.WriteTo(p, serverAddr.String())

		b := buffer.New()
		err = udpConn.nfd.FillToBuffer(b)
		assert.Nil(t, err)

		block, _ := b.ReadBlock()
		s, addr, err := getUDPDataAndAddr(block)
		assert.Nil(t, err)
		assert.Equal(t, p, s)
		assert.Equal(t, clientAddr, addr)
	})
}

func Test_netFD_err(t *testing.T) {
	rawConn, err := rawListenUDP("udp4")
	assert.Nil(t, err)
	defer rawConn.Close()
	nfd, err := rawToNetFD(rawConn)
	assert.Nil(t, err)
	t.Run("writeTo err", func(t *testing.T) {
		_, err = nfd.WriteTo(helloWorld, nil)
		assert.NotNil(t, err)

		clientAddr := "127.0.0.1:0"
		udpAddr, _ := net.ResolveUDPAddr("udp", clientAddr)
		p := make([]byte, defaultUDPBufferSize+1)
		_, err = nfd.WriteTo(p, udpAddr)
		assert.NotNil(t, err)
	})
	t.Run("syscallMMsg err", func(t *testing.T) {
		_, err = nfd.syscallMMsg(-1, nil)
		assert.NotNil(t, err)
	})
	t.Run("syscallMsg err", func(t *testing.T) {
		_, err = nfd.syscallMsg(-1, nil, 0)
		assert.NotNil(t, err)
	})
}

func Test_buildMMsgs_err(t *testing.T) {
	err := buildMMsgs([]systype.MMsghdr{{}}, [][]byte{make([]byte, 0)})
	assert.NotNil(t, err)
}

func Test_buildMsg_err(t *testing.T) {
	err := buildMsg(&systype.Msghdr{}, make([]byte, 0))
	assert.NotNil(t, err)
}
