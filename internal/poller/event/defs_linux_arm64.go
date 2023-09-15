// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by THL A29 Limited ("Tencent Modifications").
// All Tencent Modifications are Copyright (C) 2023 THL A29 Limited.

package event

// EpollEvent defines epoll event data.
type EpollEvent struct {
	Events uint32
	_pad   uint32
	Data   [8]byte // to match amd64
}
