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

//go:build 386 || arm || wasm || mips || mipsle
// +build 386 arm wasm mips mipsle

package iovec

import (
	"golang.org/x/sys/unix"
)

// SetIOVec sets the fields of IOVec according to ByteVec and the given slice count.
// It returns the actual number of IOVec filled.
func (d *IOData) SetIOVec(num int) {
	d.IOVec = d.IOVec[:0]
	for i := 0; i < num; i++ {
		d.IOVec = append(d.IOVec, unix.Iovec{
			Base: &d.ByteVec[i][0],
			Len:  uint32(len(d.ByteVec[i])),
		})
	}
}
