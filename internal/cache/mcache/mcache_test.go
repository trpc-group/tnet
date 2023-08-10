// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package mcache_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/cache/mcache"
)

func Test_malloc(t *testing.T) {
	s := mcache.Malloc(400)
	defer mcache.Free(s)
	assert.Equal(t, 400, len(s))
	assert.Equal(t, 512, cap(s))
	s = mcache.Malloc(4096)
	defer mcache.Free(s)
	assert.Equal(t, 4096, len(s))
	assert.Equal(t, 4096, cap(s))
	bigLen := 1024 * 1024 * 256
	bigCap := 1024 * 1024 * 512
	s = mcache.Malloc(bigLen, bigCap)
	defer mcache.Free(s)
	assert.Equal(t, bigLen, len(s))
	assert.Equal(t, bigCap, cap(s))
}

func Test_calIndex(t *testing.T) {
	n := mcache.CalIndex(0)
	assert.Equal(t, 0, n)
	n = mcache.CalIndex(1)
	assert.Equal(t, 1, n)
	n = mcache.CalIndex(5)
	assert.Equal(t, 3, n)
	n = mcache.CalIndex(4096)
	assert.Equal(t, math.Pow(2, float64(n)), float64(4096))
}

func BenchmarkNormal4096(b *testing.B) {
	var s []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = make([]byte, 0, 4096)
	}
	_ = s
}

func BenchmarkCache4096(b *testing.B) {
	var s []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = mcache.Malloc(4096)
		mcache.Free(s)
	}
	_ = s
}

func BenchmarkNormal10M(b *testing.B) {
	var s []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = make([]byte, 0, 1024*1024*10)
	}
	_ = s
}

func BenchmarkMCache10M(b *testing.B) {
	var s []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = mcache.Malloc(1024 * 1024 * 10)
		mcache.Free(s)
	}
	_ = s
}

func BenchmarkNormal4096Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s = make([]byte, 0, 4096)
			}
		}
		_ = s
	})
}

func BenchmarkMCache4096Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s = mcache.Malloc(4096)
				mcache.Free(s)
			}
		}
		_ = s
	})
}

func BenchmarkNormal10MParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s = make([]byte, 0, 1024*1024*10)
			}
		}
		_ = s
	})
}

func BenchmarkMCache10MParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s = mcache.Malloc(1024 * 1024 * 10)
				mcache.Free(s)
			}
		}
		_ = s
	})
}
