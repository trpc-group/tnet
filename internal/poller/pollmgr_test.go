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

package poller_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/poller"
)

func TestPollMgr(t *testing.T) {
	assert.Nil(t, poller.SetNumPollers(2))
	assert.Equal(t, 2, poller.NumPollers())

	pollmgr, err := poller.NewPollMgr("UnknownLB", 1)
	assert.NotNil(t, err)
	assert.Nil(t, pollmgr)

	pollmgr, err = poller.NewPollMgr(poller.RoundRobin, 0)
	assert.NotNil(t, err)
	assert.Nil(t, pollmgr)

	pollmgr, err = poller.NewPollMgr(poller.RoundRobin, 1)
	assert.Nil(t, err)
	assert.NotNil(t, pollmgr)

	assert.Nil(t, pollmgr.SetNumPollers(2))
	assert.Equal(t, 2, pollmgr.NumPollers())
	assert.NotNil(t, pollmgr.SetNumPollers(1))

	p := pollmgr.Pick()
	assert.NotNil(t, p)
	err = p.Trigger(func() error { return nil })
	assert.Nil(t, err)

	desc := poller.NewDesc()
	assert.NotNil(t, desc)
	defer poller.FreeDesc(desc)

	assert.Nil(t, pollmgr.Close())
}
