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

// Package mcache provides cache for byte slice.
package mcache

import (
	"sync"
)

// 2**24 > 10M
const maxSize = 25

// caches is the pool for byte slice.
// caches[i] stores slice of size 2**i.
var caches [maxSize]sync.Pool

func init() {
	for i := 0; i < maxSize; i++ {
		size := 1 << i
		caches[i].New = func() any {
			s := make([]byte, 0, size)
			return s
		}
	}
}

// Malloc creates a byte slice with the same parameters as make.
// Recycle the slice with Free.
func Malloc(size int, capacity ...int) []byte {
	if len(capacity) > 1 {
		panic("too many arguments to malloc")
	}
	c := size
	if len(capacity) > 0 && capacity[0] > size {
		c = capacity[0]
	}
	idx := CalIndex(c)
	if idx >= maxSize {
		return make([]byte, size, c)
	}
	ret := caches[idx].Get().([]byte)
	ret = ret[:size]
	return ret
}

// Free recycles a byte slice.
func Free(p []byte) {
	c := cap(p)
	if !isPowerOfTwo(c) {
		return
	}
	idx := CalIndex(c)
	if idx >= maxSize {
		return
	}
	p = p[:0]
	caches[idx].Put(p)
}

// CalIndex returns the power of two index of the given capacity.
func CalIndex(capacity int) int {
	if capacity == 0 {
		return 0
	}
	idx := log2(capacity)
	if capacity != 1 && isPowerOfTwo(capacity) {
		return idx
	}
	return idx + 1
}

func log2(x int) int {
	r := 0
	for x != 0 {
		x = x >> 1
		r++
	}
	return r - 1
}

func isPowerOfTwo(x int) bool {
	return (x & (x - 1)) == 0
}
