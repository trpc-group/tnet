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

package poller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_descCache(t *testing.T) {
	dc := &descCache{
		cache: make([]*Desc, 0, 16),
	}
	d := dc.alloc()
	require.NotNil(t, d)
	d.FD = 1
	dc.markFree(d)
	require.Equal(t, 1, d.FD)
	dc.free()
	require.Zero(t, d.FD)
}
