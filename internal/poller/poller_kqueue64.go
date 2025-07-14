//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 Tencent.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

//go:build (freebsd || dragonfly || darwin) && (amd64 || arm64)
// +build freebsd dragonfly darwin
// +build amd64 arm64

package poller

func newKeventIdent(i int) uint64 {
	return uint64(i)
}
