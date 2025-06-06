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

package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombinedWritesOptions(t *testing.T) {
	t.Run("server option", func(t *testing.T) {
		opts := &serverOptions{}
		WithServerCombinedWrites(true)(opts)
		assert.True(t, opts.combineWrites)

		WithServerCombinedWrites(false)(opts)
		assert.False(t, opts.combineWrites)
	})

	t.Run("client option", func(t *testing.T) {
		opts := &clientOptions{}
		WithClientCombinedWrites(true)(opts)
		assert.True(t, opts.combineWrites)

		WithClientCombinedWrites(false)(opts)
		assert.False(t, opts.combineWrites)
	})
}
