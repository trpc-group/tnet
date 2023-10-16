// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package websocket

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnSetErr(t *testing.T) {
	var rc rawConnection
	c := &conn{
		raw: rc,
	}
	require.NotNil(t, c.SetOnRequest(nil))
	require.NotNil(t, c.SetOnClosed(func(c Conn) error { return nil }))
}
