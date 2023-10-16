// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package safejob_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/safejob"
)

func TestOnceJob(t *testing.T) {
	job := &safejob.OnceJob{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		job.Begin()
		job.End()
		wg.Done()
	}()
	wg.Wait()
	assert.Equal(t, true, job.Closed())
}

func TestOnceJobClose(t *testing.T) {
	job := &safejob.OnceJob{}
	assert.Equal(t, false, job.Closed())
	job.Close()
	assert.Equal(t, true, job.Closed())
	assert.Equal(t, false, job.Begin())
}
