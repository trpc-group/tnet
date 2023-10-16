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

package tnet_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet"
)

func TestTCPServiceNoListener(t *testing.T) {
	_, err := tnet.NewTCPService(nil, nil)
	assert.NotNil(t, err)
}

func TestTCPServiceAcceptAndCancel(t *testing.T) {
	ln, err := tnet.Listen("tcp", "127.0.0.1:9999")
	assert.Nil(t, err)

	s, err := tnet.NewTCPService(ln, nil)
	assert.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s.Serve(ctx)
		wg.Done()
	}()
	time.Sleep(time.Millisecond * 5)
	assert.NotNil(t, s.Serve(ctx))
	client, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	assert.Nil(t, err)
	defer client.Close()

	time.Sleep(time.Millisecond * 5)
	cancel()
	wg.Wait()
}

func TestTCPServiceExternalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:9999")
	assert.Nil(t, err)
	defer ln.Close()

	s, err := tnet.NewTCPService(ln, nil)
	assert.Nil(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s.Serve(ctx)
		wg.Done()
	}()
	time.Sleep(time.Millisecond * 5)
	assert.NotNil(t, s.Serve(ctx))
	client, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	assert.Nil(t, err)
	defer client.Close()

	time.Sleep(time.Millisecond * 5)
	cancel()
	wg.Wait()
}
