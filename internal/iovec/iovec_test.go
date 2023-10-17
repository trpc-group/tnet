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

package iovec_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/internal/iovec"
)

func TestIOVEC(t *testing.T) {
	ioData := iovec.NewIOData(iovec.WithLength(iovec.DefaultLength))
	ioData.ByteVec = [][]byte{
		[]byte("test"),
	}
	length := len(ioData.ByteVec)
	ioData.SetIOVec(length)
	require.Equal(t, length, len(ioData.IOVec))
	ioData.Release(length)
	ioData.Reset()
}
