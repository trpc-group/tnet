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
	"go.uber.org/atomic"
)

// OnceJob means that the job can only be executed once and then marked closed.
type OnceJob struct {
	closed atomic.Bool
}

// Begin sets the start entry of the job to make sure it's concurrent-safe.
func (j *OnceJob) Begin() bool {
	return j.closed.CAS(false, true)
}

// End sets the end entry of the job to make sure it's concurrent-safe.
func (j *OnceJob) End() {}

// Close closes the job. After closed, the job can't be called anymore.
func (j *OnceJob) Close() {
	j.closed.Store(true)
}

// Closed returns whether the job is closed.
func (j *OnceJob) Closed() bool {
	return j.closed.Load()
}
