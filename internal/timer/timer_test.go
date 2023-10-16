// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package timer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/timer"
)

func TestTimerNormal(t *testing.T) {
	t1 := timer.New(time.Now().Add(time.Millisecond * 10))
	assert.NotNil(t, t1)
	assert.Equal(t, false, t1.Expired())
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, true, t1.Expired())

	t1.Reset(time.Time{})
	assert.Equal(t, true, t1.IsZero())
	assert.Equal(t, false, t1.Expired())

	t1.Reset(time.Now().Add(time.Millisecond * 10))
	t1.Start()
	time.Sleep(time.Millisecond * 5)
	t1.Start()
	<-t1.Wait()
	assert.Equal(t, true, t1.Expired())

	t1.Reset(time.Now().Add(time.Millisecond * 2))
	t1.Start()
	time.Sleep(time.Millisecond * 5)
	t1.Stop()
	assert.Equal(t, true, t1.IsZero())
}
