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
	"errors"
	"sync"

	"trpc.group/trpc-go/tnet/internal/iovec"
)

// NewDesc allocates a Desc for file descriptor in general.
func NewDesc() *Desc {
	return alloc()
}

// FreeDesc frees a Desc object, as the memory is managed by Poller
// system, without FreeDesc() will cause memory leak.
func FreeDesc(desc *Desc) {
	markDescFree(desc)
}

// Desc provides the fd and event callbacks, which are used by poller to
// monitor events(such as readable, writable or hang up). When event is ready,
// the poller will invoke the callback.
type Desc struct {
	mu     sync.RWMutex
	next   *Desc
	poller Poller
	index  int32
	Data   interface{}

	// Desc provides three callbacks for fd's reading, writing or hanging events.
	OnRead  func(data interface{}, ioData *iovec.IOData) error
	OnWrite func(data interface{}) error
	OnHup   func(data interface{})

	// FD is the file descriptor that will be monitored by poller.
	FD int
}

// RLock locks the Desc for reading.
func (p *Desc) RLock() {
	p.mu.RLock()
}

// RUnlock unlocks the Desc for reading.
func (p *Desc) RUnlock() {
	p.mu.RUnlock()
}

// Lock locks the Desc for reading and writing.
func (p *Desc) Lock() {
	p.mu.Lock()
}

// Unlock unlocks the Desc for reading and writing.
func (p *Desc) Unlock() {
	p.mu.Unlock()
}

// PickPoller binds the Desc to one poller that is from the default pollmgr.
func (p *Desc) PickPoller() error {
	return p.PickPollerWithPollMgr(defaultMgr)
}

// PickPollerWithPollMgr binds the Desc to one poller that is from the specified pollmgr
func (p *Desc) PickPollerWithPollMgr(mgr *PollMgr) error {
	if p.poller != nil {
		return errors.New("already bind to poller")
	}
	if mgr == nil {
		return errors.New("pollMgr is nil")
	}
	p.poller = mgr.Pick()
	return nil
}

// Control registers the event that the Desc asks poller to monitor.
func (p *Desc) Control(event Event) error {
	if p.poller == nil {
		return errors.New("invalid Desc")
	}
	return p.poller.Control(p, event)
}

// Close closes the Desc.
func (p *Desc) Close() error {
	return p.poller.Control(p, Detach)
}

func (p *Desc) reset() {
	p.FD = 0
	p.Data = nil
	p.OnRead, p.OnWrite, p.OnHup = nil, nil, nil
	p.poller = nil
}
