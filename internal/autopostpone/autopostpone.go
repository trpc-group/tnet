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

// Package autopostpone provides utilities to decide whether to postpone write.
package autopostpone

import (
	"math"

	"go.uber.org/atomic"
	"trpc.group/trpc-go/tnet/metrics"
)

const (
	// consecutiveSamePacketsNumThresh is the threshold to disable postpone write.
	consecutiveSamePacketsNumThresh = 70
	// loopCntThresh is the threshold to enable postpone write in non-blocking mode.
	loopCntThresh = 3
	// readingLockContentionThresh is the threshold to enable postpone write.
	readingLockContentionThresh = 5
)

// PostponeWrite is the struct for postpone write.
type PostponeWrite struct {
	prevPacketsNum                  int
	readingTryLockFailed            atomic.Uint32
	loopCnt                         uint8
	consecutiveSamePacketsNumCounts uint8
	enable                          bool
}

// CheckAndDisablePostponeWrite changes postpone write to false if max consecutive same packets number is reached.
func (f *PostponeWrite) CheckAndDisablePostponeWrite(packetsNum int) {
	defer func() { f.prevPacketsNum = packetsNum }()
	if f.prevPacketsNum != 0 && f.prevPacketsNum != packetsNum {
		f.consecutiveSamePacketsNumCounts = 0
		return
	}
	f.consecutiveSamePacketsNumCounts++
	if f.consecutiveSamePacketsNumCounts >= consecutiveSamePacketsNumThresh {
		f.consecutiveSamePacketsNumCounts = 0
		f.enable = false
		metrics.Add(metrics.TCPPostponeWriteOff, 1)
	}
}

// Set sets the postpone write value manually.
func (f *PostponeWrite) Set(postponeWrite bool) {
	f.enable = postponeWrite
}

// Enabled returns whether postpone write is enabled.
func (f *PostponeWrite) Enabled() bool {
	return f.enable
}

// ResetLoopCnt resets loop count to 0.
func (f *PostponeWrite) ResetLoopCnt() {
	f.loopCnt = 0
}

// IncLoopCnt increments loop count.
func (f *PostponeWrite) IncLoopCnt() {
	if f.loopCnt < math.MaxUint8 {
		f.loopCnt++
	}
}

// CheckLoopCnt changes postpone write to true if user handler is processed more than a certain threshold.
func (f *PostponeWrite) CheckLoopCnt() {
	if f.loopCnt > loopCntThresh {
		f.enable = true
		metrics.Add(metrics.TCPPostponeWriteOn, 1)
	}
}

// ResetReadingTryLockFail resets reading try lock failed counts.
func (f *PostponeWrite) ResetReadingTryLockFail() {
	f.readingTryLockFailed.Store(0)
}

// IncReadingTryLockFail increases reading try lock failed counts and changes postpone write to true
// if high reading lock contention is observed.
func (f *PostponeWrite) IncReadingTryLockFail() {
	// contentionThresh is chosen according to experimentation.
	// when:
	//   1. postponeWrite=false and no concurrent packets on a single connection
	//   2. postponeWrite=true and lots of concurrent packets on a single connection (multiplexing scenario)
	// the failed count is almost always 0 or 1.
	// HOWEVER, when:
	//   * postponeWrite=false and lots of concurrent packets on a single connection (multiplexing scenario)
	// the failed count can be very high (>10).
	if c := f.readingTryLockFailed.Add(1); c > readingLockContentionThresh {
		f.enable = true
		metrics.Add(metrics.TCPPostponeWriteOn, 1)
		f.ResetReadingTryLockFail()
	}
}
