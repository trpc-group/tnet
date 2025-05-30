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

package systype_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
)

func TestGetIOHdr(t *testing.T) {
	bs := make([][]byte, 10)
	for i := 0; i < len(bs); i++ {
		bs[i] = []byte("a")
	}
	iovecs, hdr := systype.GetIOVECWrapper(bs)
	if hdr != nil {
		defer systype.PutIOVECWrapper(hdr)
	}
	assert.Equal(t, 10, len(iovecs))
	assert.Equal(t, systype.MaxLen, cap(iovecs))

	bs = make([][]byte, systype.MaxLen+1)
	for i := 0; i < len(bs); i++ {
		bs[i] = []byte("a")
	}
	bigIovecs, w := systype.GetIOVECWrapper(bs)
	assert.Nil(t, w)
	assert.Equal(t, systype.MaxLen+1, len(bigIovecs))
}

func TestGetIOData(t *testing.T) {
	iovs, w := systype.GetIOData(10)
	defer systype.PutIOData(w)
	assert.Equal(t, 10, len(iovs))
	assert.Equal(t, systype.MaxLen, cap(iovs))

	bigIovs, w := systype.GetIOData(systype.MaxLen + 1)
	assert.Nil(t, w)
	assert.Equal(t, systype.MaxLen+1, len(bigIovs))
}

func TestBuildMMsg(t *testing.T) {
	m := systype.MMsghdr{}
	systype.BuildMMsg(&m, []byte("name"), []byte("buffer"))
	assert.NotNil(t, m.Hdr.Iov)
}

func TestBuildMsg(t *testing.T) {
	m := systype.Msghdr{}
	systype.BuildMsg(&m, []byte("name"), []byte("buffer"))
	assert.NotNil(t, m.Iov)
}

func BenchmarkNormal20(b *testing.B) {
	var s [][]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = make([][]byte, 0, 20)
	}
	_ = s
}

func BenchmarkCache20(b *testing.B) {
	var s [][]byte
	var w *systype.IOData
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, w = systype.GetIOData(20)
		systype.PutIOData(w)
	}
	_ = s
}

func BenchmarkNormal20Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s [][]byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s = make([][]byte, 20)
			}
		}
		_ = s
	})
}

func BenchmarkMCache20Parallel(b *testing.B) {
	var w *systype.IOData
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s [][]byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				s, w = systype.GetIOData(20)
				systype.PutIOData(w)
			}
		}
		_ = s
	})
}

func TestGetMMsghdrs(t *testing.T) {
	ms := systype.GetMMsghdrs(10)
	defer systype.PutMMsghdrs(ms)
	assert.Equal(t, 10, len(ms))
	assert.Equal(t, systype.MaxLen, cap(ms))

	bigms := systype.GetMMsghdrs(systype.MaxLen + 1)
	defer systype.PutMMsghdrs(bigms)
	assert.Equal(t, systype.MaxLen+1, len(bigms))
}

func TestGetMsghdr(t *testing.T) {
	msg := systype.GetMsghdr()
	defer systype.PutMsghdr(msg)
	assert.NotNil(t, msg)
}
