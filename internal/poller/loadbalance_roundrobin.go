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

package poller

import (
	"sync/atomic"
)

// RoundRobin denotes the name of loadbalance.
const RoundRobin string = "RoundRobinLB"

func init() {
	RegisterBalanceBuilder(RoundRobin, func() LoadBalance { return &roundRobinLB{} })
}

type roundRobinLB struct {
	pollers  []Poller
	accepted uintptr
	pollSize int
}

// Name returns loadbalance type.
func (r *roundRobinLB) Name() string {
	return RoundRobin
}

// Register registers poller to loadbalance.
func (r *roundRobinLB) Register(poller Poller) {
	r.pollers = append(r.pollers, poller)
	r.pollSize++
}

// Pick picks poller according to loadbalance algorithm.
func (r *roundRobinLB) Pick() Poller {
	idx := int(atomic.AddUintptr(&r.accepted, 1)) % r.pollSize
	return r.pollers[idx]
}

// Len returns pollers size.
func (r *roundRobinLB) Len() int {
	return r.pollSize
}

// Iterate iterates the pollers and invokes function f, if f returns false, iteration will stop.
func (r *roundRobinLB) Iterate(f func(int, Poller) bool) {
	for index, poller := range r.pollers {
		if !f(index, poller) {
			break
		}
	}
}
