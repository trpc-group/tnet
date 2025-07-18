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
