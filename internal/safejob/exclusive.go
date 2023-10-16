// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package safejob

import (
	"sync"

	"go.uber.org/atomic"
	"trpc.group/trpc-go/tnet/internal/locker"
)

// ExclusiveUnblockJob executes job exclusively, if control is not acquired, directly return.
type ExclusiveUnblockJob struct {
	l      locker.Locker
	closed atomic.Bool
}

// Begin sets the start entry of the job.
func (j *ExclusiveUnblockJob) Begin() bool {
	if !j.l.TryLock() {
		return false
	}
	if j.closed.Load() {
		j.l.Unlock()
		return false
	}
	return true
}

// End sets the end entry of the job.
func (j *ExclusiveUnblockJob) End() {
	j.l.Unlock()
}

// Close the job, after closed the job can't be executed anymore.
func (j *ExclusiveUnblockJob) Close() {
	j.l.Lock()
	j.closed.Store(true)
	j.l.Unlock()
}

// Closed returns whether the job is closed.
func (j *ExclusiveUnblockJob) Closed() bool {
	return j.closed.Load()
}

// ExclusiveBlockJob executes the job exclusively, waiting for acquiring the job control.
type ExclusiveBlockJob struct {
	mu     sync.Mutex
	closed atomic.Bool
}

// Begin sets the start entry of the job to make sure it's concurrent-safe.
func (j *ExclusiveBlockJob) Begin() bool {
	j.mu.Lock()
	if j.closed.Load() {
		j.mu.Unlock()
		return false
	}
	return true
}

// End sets the end entry of the job to make sure it's concurrent-safe.
func (j *ExclusiveBlockJob) End() {
	j.mu.Unlock()
}

// Close the job, after closed the job can't be called anymore.
func (j *ExclusiveBlockJob) Close() {
	j.mu.Lock()
	j.closed.Store(true)
	j.mu.Unlock()
}

// Closed returns whether the job is closed.
func (j *ExclusiveBlockJob) Closed() bool {
	return j.closed.Load()
}
