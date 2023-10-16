// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

// Package tls provides tls connection utilities.
package tls

import (
	"net"
	"time"
)

// Conn defines tls connection interface.
type Conn interface {
	net.Conn
	// SetMetaData sets meta data. Through this method, users can bind some custom data to a connection.
	SetMetaData(interface{})
	// GetMetaData gets meta data.
	GetMetaData() interface{}
	// SetIdleTimeout sets connection level idle timeout.
	SetIdleTimeout(d time.Duration) error
	// SetFlushWrite sets flush write flag for the connection.
	SetFlushWrite(flushWrite bool)
	// SetOnRequest can set or replace the tls.Handler method for a connection.
	SetOnRequest(handle Handler) error
	// SetOnClosed sets the additional close process for a connection.
	SetOnClosed(handle Handler) error
	// IsActive checks whether the connection is active or not.
	IsActive() bool
}
