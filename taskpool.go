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

package tnet

import (
	"github.com/panjf2000/ants/v2"
	"trpc.group/trpc-go/tnet/metrics"
)

var (
	maxRoutines = 0 // meaning INT32_MAX.
	sysPool, _  = ants.NewPoolWithFunc(maxRoutines, taskHandler)
	usrPool, _  = ants.NewPool(maxRoutines)
)

func taskHandler(v any) {
	switch conn := v.(type) {
	case *tcpconn:
		tcpAsyncHandler(conn)
	case *udpconn:
		udpAsyncHandler(conn)
	}
}

func doTask(args any) error {
	metrics.Add(metrics.TaskAssigned, 1)
	return sysPool.Invoke(args)
}

// Submit submits a task to usrPool.
//
// Users can use this API to submit a task to
// the default user business goroutine pool.
func Submit(task func()) error {
	return usrPool.Submit(task)
}
