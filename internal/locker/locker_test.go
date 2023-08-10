// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package locker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/locker"
)

func TestLocker(t *testing.T) {
	l := locker.New()
	assert.Equal(t, false, l.IsLocked())
	l.Lock()
	assert.Equal(t, true, l.IsLocked())
	assert.Equal(t, false, l.TryLock())
	l.Unlock()
	assert.Equal(t, false, l.IsLocked())

	assert.Equal(t, true, l.TryLock())
	assert.Equal(t, true, l.IsLocked())
	l.Unlock()
	assert.Equal(t, false, l.IsLocked())
}

func HammerMutex(t *testing.T, l *locker.Locker, loops int, cdone chan bool) {
	for i := 0; i < loops; i++ {
		l.Lock()
		assert.Equal(t, true, l.IsLocked())
		l.Unlock()
	}
	cdone <- true
}

func TestConCurrentLocker(t *testing.T) {
	l := locker.New()
	c := make(chan bool)
	for i := 0; i < 10; i++ {
		go HammerMutex(t, l, 1000, c)
	}
	for i := 0; i < 10; i++ {
		<-c
	}
	assert.Equal(t, false, l.IsLocked())
}
