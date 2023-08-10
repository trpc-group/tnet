// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package poller

import (
	"fmt"
)

var defaultMgr *PollMgr

func init() {
	pollmgr, err := NewPollMgr(RoundRobin, 1)
	if err != nil {
		panic(err)
	}
	defaultMgr = pollmgr
}

// NewPollMgr creates a PollMgr object.
func NewPollMgr(balance string, loops int, opts ...Option) (*PollMgr, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	builder := GetBalanceBuilder(balance)
	if builder == nil {
		return nil, fmt.Errorf("loadbalance %s is not registered", balance)
	}
	pollmgr := &PollMgr{lb: builder(), ignoreTaskError: o.ignoreTaskError}
	if err := pollmgr.SetNumPollers(loops); err != nil {
		return nil, err
	}
	return pollmgr, nil
}

type options struct {
	ignoreTaskError bool
}

// Option provides poller manager option.
type Option func(*options)

// WithIgnoreTaskError sets the boolean value of ignore task error.
func WithIgnoreTaskError(ignore bool) Option {
	return func(o *options) {
		o.ignoreTaskError = ignore
	}
}

// SetNumPollers is used to set the number of pollers. Generally it is not actively used.
// N can't be smaller than the current poller numbers. Default poller number is 1.
func SetNumPollers(n int) error {
	return defaultMgr.SetNumPollers(n)
}

// NumPollers returns the number of pollers.
func NumPollers() int {
	return defaultMgr.NumPollers()
}

// PollMgr is used to manage all the pollers, including scaling out pollers and
// asking loadbalance to pick poller for Desc.
type PollMgr struct {
	lb              LoadBalance
	ignoreTaskError bool
}

// SetNumPollers scales up the pollers.
func (pm *PollMgr) SetNumPollers(loops int) error {
	if loops == 0 || loops < pm.lb.Len() {
		return fmt.Errorf("loops can't be smaller than current loops[%d]", pm.lb.Len())
	}
	pm.run(loops)
	return nil
}

// NumPollers returns pollers number of pollMgr.
func (pm *PollMgr) NumPollers() int {
	return pm.lb.Len()
}

// Pick ask loadbalance to pick poller for Desc.
func (pm *PollMgr) Pick() Poller {
	return pm.lb.Pick()
}

// Close closes all the pollers managed by PollMgr.
func (pm *PollMgr) Close() error {
	pm.lb.Iterate(func(_ int, poller Poller) bool {
		_ = poller.Close()
		return true
	})
	return nil
}

func (pm *PollMgr) run(loops int) {
	for i := pm.lb.Len(); i < loops; i++ {
		poller, err := newPoller(pm.ignoreTaskError)
		if err != nil {
			panic(err)
		}
		pm.lb.Register(poller)
		go poller.Wait()
	}
}
