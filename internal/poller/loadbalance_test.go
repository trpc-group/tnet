// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

package poller_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/tnet/internal/poller"
)

const fakeLoadbalance string = "FakeLB"

type fakeLB struct {
}

func (r *fakeLB) Name() string {
	return fakeLoadbalance
}

func (r *fakeLB) Register(p poller.Poller) {
}

func (r *fakeLB) Pick() poller.Poller {
	return nil
}

func (r *fakeLB) Len() int {
	return 0
}

func (r *fakeLB) Iterate(f func(int, poller.Poller) bool) {
}

func TestRegisterLoadbalance(t *testing.T) {
	poller.RegisterBalanceBuilder(fakeLoadbalance, func() poller.LoadBalance {
		return &fakeLB{}
	})
	build := poller.GetBalanceBuilder(fakeLoadbalance)
	assert.NotNil(t, build)
}
