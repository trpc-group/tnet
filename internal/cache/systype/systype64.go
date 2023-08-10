// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

//go:build amd64 || arm64 || mips64 || mips64le || ppc64 || ppc64le || riscv64 || s390x || loong64
// +build amd64 arm64 mips64 mips64le ppc64 ppc64le riscv64 s390x loong64

package systype

func converUint(i int) uint64 {
	return uint64(i)
}
