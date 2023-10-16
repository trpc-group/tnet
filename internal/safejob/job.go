// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

// Package safejob provides functions to call job in a concurrent-safe manner.
package safejob

// Job defines the interface that can call job multiple times and ensure concurrent safety.
type Job interface {
	// Begin sets the start entry of the job.
	Begin() bool

	// End sets the end entry of the job.
	End()

	// Close closes the job. After closed, the job can't be executed anymore.
	Close()

	// Closed returns whether the job is closed.
	Closed() bool
}
