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

package stat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReporter(t *testing.T) {
	Report(ClientAttr, TCPAttr)
}

func TestGetEnvVar(t *testing.T) {
	assert.NotEmpty(t, getEnvVar("Test"))
}
