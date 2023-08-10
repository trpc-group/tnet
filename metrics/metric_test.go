// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package metrics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/metrics"
)

func TestMetrics(t *testing.T) {
	metrics.Add(metrics.TCPReadvCalls, 1)
	assert.Equal(t, uint64(1), metrics.Get(metrics.TCPReadvCalls))
	metrics.Add(metrics.TCPReadvCalls, 1)
	assert.Equal(t, uint64(2), metrics.Get(metrics.TCPReadvCalls))
	metrics.Add(metrics.Max+1, 1)
	metrics.Add(metrics.EpollNoWait, 8)
	metrics.Add(metrics.EpollWait, 9)
	metrics.Add(metrics.EpollEvents, 99)
	metrics.Add(metrics.TCPWritevCalls, 191)
	metrics.Add(metrics.TCPWritevBlocks, 1191)
	metrics.Add(metrics.TCPReadvCalls, 191)
	metrics.Add(metrics.TCPReadvBytes, 1191)
	assert.Equal(t, uint64(0), metrics.Get(metrics.Max+1))
	metrics.ShowMetrics()
	metrics.ShowMetricsOfPeriod(time.Millisecond)
}
