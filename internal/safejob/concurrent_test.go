// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package safejob_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/safejob"
)

func TestConcurrentJob(t *testing.T) {
	job := &safejob.ConcurrentJob{}
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			assert.Equal(t, true, job.Begin())
			time.Sleep(time.Millisecond)
			job.End()
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, false, job.Closed())
}

func TestConcurrentJobClose(t *testing.T) {
	job := &safejob.ConcurrentJob{}
	assert.Equal(t, false, job.Closed())
	job.Close()
	assert.Equal(t, true, job.Closed())
	assert.Equal(t, false, job.Begin())
}
