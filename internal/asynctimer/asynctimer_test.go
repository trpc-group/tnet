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

package asynctimer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/asynctimer"
)

type testWrapper struct {
	begin     time.Time
	call      time.Time
	isHandled bool
}

var expireHandle = func(data any) {
	t, ok := data.(*testWrapper)
	if !ok {
		return
	}
	t.call = time.Now()
	t.isHandled = true
}

func TestNewTimeWheel(t *testing.T) {
	timeUnit := 10 * time.Millisecond
	tw, err := asynctimer.NewTimeWheel(timeUnit, 3)
	assert.Nil(t, err)
	tw.Start()
	defer tw.Stop()

	timeout := 5 * timeUnit
	data := &testWrapper{begin: time.Now()}
	timer := asynctimer.NewTimer(data, expireHandle, timeout)
	err = tw.Add(timer)
	assert.Nil(t, err)

	time.Sleep(timeout + timeUnit)
	realTimeout := data.call.Sub(data.begin)
	assert.GreaterOrEqual(t, realTimeout, timeout-timeUnit)
	assert.LessOrEqual(t, realTimeout, timeout+timeUnit)
}

func TestDefaultTimeWheel(t *testing.T) {
	a, b := &testWrapper{}, &testWrapper{}
	ta := asynctimer.NewTimer(a, expireHandle, time.Second)
	tb := asynctimer.NewTimer(a, expireHandle, time.Second)
	err := asynctimer.Add(ta)
	assert.Nil(t, err)
	err = asynctimer.Add(tb)
	assert.Nil(t, err)
	assert.False(t, a.isHandled)
	assert.False(t, b.isHandled)

	time.Sleep(500 * time.Millisecond)
	asynctimer.Del(tb)
	time.Sleep(500 * time.Millisecond)
	assert.True(t, a.isHandled)
	assert.False(t, b.isHandled)
}

func TestTimeWheelRefresh(t *testing.T) {
	timeUnit := 10 * time.Millisecond
	tw, err := asynctimer.NewTimeWheel(timeUnit, 3)
	assert.Nil(t, err)
	tw.Start()
	defer tw.Stop()

	timeout := 8 * timeUnit
	data := &testWrapper{begin: time.Now()}
	timer := asynctimer.NewTimer(data, expireHandle, timeout)
	err = tw.Add(timer)
	assert.Nil(t, err)

	refreshIntervel := 4 * timeUnit
	go func() {
		time.Sleep(refreshIntervel)
		tw.Add(timer)
	}()

	time.Sleep(timeout + timeUnit)
	assert.False(t, data.isHandled)

	time.Sleep(refreshIntervel)
	timeout += refreshIntervel
	realTimeout := data.call.Sub(data.begin)
	assert.True(t, data.isHandled)
	assert.GreaterOrEqual(t, realTimeout, timeout-timeUnit)
	assert.LessOrEqual(t, realTimeout, timeout+timeUnit)
}
