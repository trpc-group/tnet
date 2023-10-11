// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by THL A29 Limited ("Tencent Modifications").
// All Tencent Modifications are Copyright (C) 2023 THL A29 Limited.

//go:build linux && (mips || mipsle)

// Package event provides definitions of event data.
package event

// EpollEvent defines epoll event data.
type EpollEvent struct {
	Events    uint32
	pad_cgo_0 [4]byte
	Data      uint64
}
