// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package locker provides locking utilities.
package locker

import (
	"runtime"
	"sync/atomic"
)

const (
	unlocked = 0
	locked   = 1
)

// A Locker is a spinlock exclusion lock.
// The zero value for a Locker is unlocked.
type Locker uint32

// New creates a Locker.
func New() *Locker {
	var l Locker
	return &l
}

// Lock locks l.
// If the lock is already in use, the calling goroutine
// will block until the locker is available.
func (l *Locker) Lock() {
	for !atomic.CompareAndSwapUint32((*uint32)(l), unlocked, locked) {
		runtime.Gosched()
	}
}

// Unlock unlocks l.
// A locked Locker is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a locker and then
// arrange for another goroutine to unlock it.
func (l *Locker) Unlock() {
	atomic.StoreUint32((*uint32)(l), unlocked)
}

// TryLock tries to lock l, if the locker is already locked by others
// the calling goroutine will not blocked, and directly return false.
func (l *Locker) TryLock() bool {
	return atomic.CompareAndSwapUint32((*uint32)(l), unlocked, locked)
}

// IsLocked returns whether the locker is locked.
func (l *Locker) IsLocked() bool {
	return atomic.LoadUint32((*uint32)(l)) == locked
}

// NoopLocker represents empty locker.
type NoopLocker struct{}

// Lock implements empty lock.
func (l *NoopLocker) Lock() {}

// Unlock implements empty unlock.
func (l *NoopLocker) Unlock() {}
