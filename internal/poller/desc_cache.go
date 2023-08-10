// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package poller

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

const pollBlockSize = 4 * 1024

func init() {
	defaultDescCache = &descCache{
		cache: make([]*Desc, 0, 1024),
	}
	runtime.KeepAlive(defaultDescCache)
}

var defaultDescCache *descCache

type descCache struct {
	first  *Desc
	cache  []*Desc
	locked int32

	mu       sync.Mutex // mu protects freeList
	freeList []int32    // freelist stores the freeable desc index to reduce GC pressure.
}

func alloc() *Desc {
	return defaultDescCache.alloc()
}

func (dc *descCache) alloc() *Desc {
	dc.lock()
	if dc.first == nil {
		const pdSize = unsafe.Sizeof(Desc{})
		n := pollBlockSize / pdSize
		if n == 0 {
			n = 1
		}
		index := int32(len(dc.cache))
		// Must be in non-GC memory because can be referenced
		// only from epoll/kqueue internals.
		for i := uintptr(0); i < n; i++ {
			pd := &Desc{index: index}
			dc.cache = append(dc.cache, pd)
			pd.next = dc.first
			dc.first = pd
			index++
		}
	}
	pd := dc.first
	dc.first = pd.next
	dc.unlock()
	return pd
}

func markDescFree(pd *Desc) {
	defaultDescCache.markFree(pd)
}

func freeDesc() {
	defaultDescCache.free()
}

// markFree marks desc is free now, the desc can be recycled later.
func (dc *descCache) markFree(pd *Desc) {
	dc.mu.Lock()
	dc.freeList = append(dc.freeList, pd.index)
	dc.mu.Unlock()
}

// free frees all descs in freeList.
func (dc *descCache) free() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if len(dc.freeList) == 0 {
		return
	}

	dc.lock()
	for _, i := range dc.freeList {
		pd := dc.cache[i]
		pd.reset()
		pd.next = dc.first
		dc.first = pd
	}
	dc.freeList = dc.freeList[:0]
	dc.unlock()
}

func (dc *descCache) lock() {
	// Using spinlock instead of mutex here reduces the latency,
	// because allocating Desc is fast.
	for !atomic.CompareAndSwapInt32(&dc.locked, 0, 1) {
		runtime.Gosched()
	}
}

func (dc *descCache) unlock() {
	atomic.StoreInt32(&dc.locked, 0)
}
