// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build (freebsd || dragonfly || darwin) && (386 || arm)
// +build freebsd dragonfly darwin
// +build 386 arm

package poller

func newKeventIdent(i int) uint32 {
	return uint32(i)
}
