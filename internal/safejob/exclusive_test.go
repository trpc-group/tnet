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

package safejob_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/safejob"
)

func TestExclusiveUnblockJob(t *testing.T) {
	job := &safejob.ExclusiveUnblockJob{}
	wg := sync.WaitGroup{}
	endCh := make(chan struct{})
	wg.Add(1)
	go func() {
		assert.Equal(t, true, job.Begin())
		wg.Done()
		time.Sleep(time.Millisecond * 5)
		job.End()
		endCh <- struct{}{}
	}()
	wg.Wait()
	assert.Equal(t, false, job.Begin())
	<-endCh
	wg.Add(1)
	go func() {
		assert.Equal(t, true, job.Begin())
		wg.Done()
		job.End()
		endCh <- struct{}{}
	}()
	wg.Wait()
	<-endCh
	assert.Equal(t, false, job.Closed())
}

func TestExclusiveUnblockJobClose(t *testing.T) {
	job := &safejob.ExclusiveUnblockJob{}
	assert.Equal(t, false, job.Closed())
	job.Close()
	assert.Equal(t, true, job.Closed())
	assert.Equal(t, false, job.Begin())
}

func TestExclusiveBlockJob(t *testing.T) {
	job := &safejob.ExclusiveBlockJob{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		assert.Equal(t, true, job.Begin())
		wg.Done()
		time.Sleep(time.Millisecond * 5)
		job.End()
	}()
	wg.Wait()
	// Blocking
	assert.Equal(t, true, job.Begin())
	job.End()
	assert.Equal(t, false, job.Closed())
}

func TestExclusiveBlockJobClose(t *testing.T) {
	job := &safejob.ExclusiveBlockJob{}
	assert.Equal(t, false, job.Closed())
	job.Close()
	assert.Equal(t, true, job.Closed())
	assert.Equal(t, false, job.Begin())
}
