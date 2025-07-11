//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 Tencent.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package safejob

import (
	"sync"

	"go.uber.org/atomic"
)

// ConcurrentJob executes the job concurrently.
type ConcurrentJob struct {
	mu     sync.RWMutex
	closed atomic.Bool
}

// Begin sets the start entry of the job.
func (j *ConcurrentJob) Begin() bool {
	j.mu.RLock()
	if j.closed.Load() {
		j.mu.RUnlock()
		return false
	}
	return true
}

// End sets the end entry of the job.
func (j *ConcurrentJob) End() {
	j.mu.RUnlock()
}

// Close closes the job, after closed the job can't be called anymore.
func (j *ConcurrentJob) Close() {
	j.mu.Lock()
	j.closed.Store(true)
	j.mu.Unlock()
}

// Closed returns whether the job is closed.
func (j *ConcurrentJob) Closed() bool {
	return j.closed.Load()
}
