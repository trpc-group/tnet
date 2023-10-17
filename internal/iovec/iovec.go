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

// Package iovec provides utilities to work with unix.Iovec.
package iovec

import (
	"golang.org/x/sys/unix"
)

// DefaultLength represents default IO vector length.
const DefaultLength = 8

// IOData wraps byte slices and unix.Iovec slice.
type IOData struct {
	ByteVec [][]byte
	IOVec   []unix.Iovec
}

// NewIOData creates an iovec.IOData with vector of size iovec.DefaultLength.
func NewIOData(opt ...Option) IOData {
	opts := &options{}
	opts.setDefault()
	for _, o := range opt {
		o(opts)
	}
	return IOData{
		ByteVec: make([][]byte, opts.length),
		IOVec:   make([]unix.Iovec, opts.length),
	}
}

// IsNil returns whether this IOData hasn't been allocated with memory.
func (d *IOData) IsNil() bool {
	return d.ByteVec == nil || d.IOVec == nil
}

// Release resets pointers in byte vector and io vector to release memory.
func (d *IOData) Release(sliceCnt int) {
	if sliceCnt > len(d.ByteVec) {
		sliceCnt = len(d.ByteVec)
	}
	if sliceCnt > len(d.IOVec) {
		sliceCnt = len(d.IOVec)
	}
	for i := 0; i < sliceCnt; i++ {
		d.ByteVec[i] = nil
		d.IOVec[i].Base = nil
	}
}

// Reset resets the length of the slices to reuse memory.
func (d *IOData) Reset() {
	d.ByteVec = d.ByteVec[:0]
	d.IOVec = d.IOVec[:0]
}
