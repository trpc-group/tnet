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

package poller_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/poller"
)

func TestPollDesc(t *testing.T) {
	eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	require.Nil(t, err)
	defer unix.Close(eventFD)
	desc := poller.NewDesc()
	desc.FD = eventFD
	assert.Nil(t, desc.PickPoller())
	assert.Nil(t, desc.Control(poller.Readable))
	assert.Nil(t, desc.Control(poller.ModWritable))
	assert.Nil(t, desc.Close())

	desc = poller.NewDesc()
	desc.FD = eventFD
	pollmgr, err := poller.NewPollMgr(poller.RoundRobin, 1)
	assert.Nil(t, err)
	assert.Nil(t, desc.PickPollerWithPollMgr(pollmgr))
	// Desc has already been bound to the poller.
	assert.NotNil(t, desc.PickPollerWithPollMgr(nil))

	desc = poller.NewDesc()
	// Mgr is nil.
	assert.NotNil(t, desc.PickPollerWithPollMgr(nil))
}

func TestNormal(t *testing.T) {
	eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	require.Nil(t, err)
	defer unix.Close(eventFD)
	var onRead, onHup int
	pollDesc := poller.NewDesc()
	pollDesc.FD = eventFD
	pollDesc.Data = 1
	ch := make(chan struct{}, 1)
	pollDesc.OnRead = func(_ interface{}, _ *iovec.IOData) error {
		onRead++
		ch <- struct{}{}
		buf := make([]byte, 8)
		unix.Read(eventFD, buf)
		return nil
	}
	hup := make(chan struct{}, 1)
	pollDesc.OnHup = func(_ interface{}) {
		onHup = 1
		hup <- struct{}{}
	}

	pollDesc.PickPoller()
	require.Nil(t, pollDesc.Control(poller.Readable))
	buf := []byte{1, 0, 0, 0, 0, 0, 0, 0}
	n, err := unix.Write(eventFD, buf)
	assert.Nil(t, err)
	assert.Equal(t, n, len(buf))
	<-ch
	assert.Equal(t, onRead, 1)
	pollDesc.OnRead = func(_ interface{}, _ *iovec.IOData) error {
		return errors.New("fake fails")
	}
	_, err = unix.Write(eventFD, buf)
	assert.Nil(t, err)
	<-hup
	assert.Equal(t, onHup, 1)
}

func TestClientClose(t *testing.T) {
	eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	require.Nil(t, err)
	pollDesc := poller.NewDesc()
	pollDesc.FD = eventFD
	require.Nil(t, pollDesc.PickPoller())
	unix.Close(eventFD)
	require.NotNil(t, pollDesc.Close())
}

func TestPollDescEvent(t *testing.T) {
	t.Run("Readable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Readable))
		assert.Nil(t, desc.Close())
	})
	t.Run("Writable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Writable))
		assert.Nil(t, desc.Close())
	})
	t.Run("ReadWriteable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.ReadWriteable))
		assert.Nil(t, desc.Close())
	})
	t.Run("ModReadable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Readable))
		assert.Nil(t, desc.Control(poller.ModReadable))
		assert.Nil(t, desc.Close())
	})
	t.Run("ModWritable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Writable))
		assert.Nil(t, desc.Control(poller.ModWritable))
		assert.Nil(t, desc.Close())
	})
	t.Run("ModReadWriteable", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Writable))
		assert.Nil(t, desc.Control(poller.ModReadWriteable))
		assert.Nil(t, desc.Close())
	})
	t.Run("Detach", func(t *testing.T) {
		eventFD, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
		require.Nil(t, err)
		defer unix.Close(eventFD)
		desc := poller.NewDesc()
		desc.FD = eventFD
		assert.Nil(t, desc.PickPoller())
		assert.Nil(t, desc.Control(poller.Writable))
		assert.Nil(t, desc.Control(poller.Detach))
	})
}
