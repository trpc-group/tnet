// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package poller

import (
	"reflect"
	"sync"
)

var (
	lbs    = make(map[string]BalanceBuilder)
	lbsMux = sync.RWMutex{}
)

// BalanceBuilder is used to create LoadBalance object.
type BalanceBuilder func() LoadBalance

// LoadBalance provides the method to pick poller from pollers.
type LoadBalance interface {
	// Name returns the name of Loadbalance.
	Name() string

	// Register registers the poller to Loadbalance.
	Register(Poller)

	// Pick picks poller according to loadbalance algorithm.
	Pick() Poller

	// Iterate iterates the pollers and invokes function f, if f returns false, iteration will stop.
	Iterate(func(int, Poller) bool)

	// Len returns pollers size.
	Len() int
}

// GetBalanceBuilder gets BalanceBuilder.
func GetBalanceBuilder(name string) BalanceBuilder {
	lbsMux.RLock()
	builder := lbs[name]
	lbsMux.RUnlock()
	return builder
}

// RegisterBalanceBuilder registers BalanceBuilder.
func RegisterBalanceBuilder(name string, builder BalanceBuilder) {
	lbv := reflect.ValueOf(builder)
	if builder == nil || lbv.Kind() == reflect.Ptr && lbv.IsNil() {
		panic("loadbalance: register nil loadbalance")
	}
	if name == "" {
		panic("loadbalance: register empty name of loadbalance")
	}
	lbsMux.Lock()
	lbs[name] = builder
	lbsMux.Unlock()
}
