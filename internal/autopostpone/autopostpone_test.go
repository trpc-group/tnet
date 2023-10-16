// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package autopostpone_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet/internal/autopostpone"
)

func TestAutopostpone(t *testing.T) {
	postpone := autopostpone.PostponeWrite{}
	require.False(t, postpone.Enabled())
	postpone.Set(true)
	require.True(t, postpone.Enabled())
	postpone.CheckAndDisablePostponeWrite(1)
	require.True(t, postpone.Enabled())
	for i := 0; i < 81; i++ {
		postpone.CheckAndDisablePostponeWrite(3)
	}
	require.False(t, postpone.Enabled())
	postpone.ResetLoopCnt()
	postpone.CheckLoopCnt()
	require.False(t, postpone.Enabled())
	for i := 0; i < 7; i++ {
		postpone.IncLoopCnt()
	}
	postpone.CheckLoopCnt()
	require.True(t, postpone.Enabled())
	postpone.Set(false)
	require.False(t, postpone.Enabled())
	postpone.ResetReadingTryLockFail()
	for i := 0; i < 7; i++ {
		postpone.IncReadingTryLockFail()
	}
	require.True(t, postpone.Enabled())
}
