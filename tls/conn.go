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

package tls

import (
	"crypto/tls"
	"time"

	"trpc.group/trpc-go/tnet"
)

// conn implements Conn.
type conn struct {
	*tls.Conn
	raw      tnet.Conn
	metaData interface{}
}

// SetMetaData sets meta data.
func (c *conn) SetMetaData(m interface{}) {
	c.metaData = m
}

// GetMetaData gets meta data.
func (c *conn) GetMetaData() interface{} {
	return c.metaData
}

// SetIdleTimeout sets connection level idle timeout.
func (c *conn) SetIdleTimeout(d time.Duration) error {
	return c.raw.SetIdleTimeout(d)
}

// SetWriteIdleTimeout sets the write idle timeout for closing the connection.
func (c *conn) SetWriteIdleTimeout(d time.Duration) error {
	return c.raw.SetWriteIdleTimeout(d)
}

// SetReadIdleTimeout sets the read idle timeout for closing the connection.
func (c *conn) SetReadIdleTimeout(d time.Duration) error {
	return c.raw.SetReadIdleTimeout(d)
}

// SetFlushWrite sets flush write flag for the connection.
func (c *conn) SetFlushWrite(flushWrite bool) {
	c.raw.SetFlushWrite(flushWrite)
}

// SetOnRequest can set or replace the tls.Handler method for a connection.
func (c *conn) SetOnRequest(handle Handler) error {
	return c.raw.SetOnRequest(func(_ tnet.Conn) error {
		// Inside the crypto/tls, there is an internal buffer to store data.
		// When the tnet buffer is empty but there is data present in the crypto/tls buffer,
		// the onRequest function does not trigger. In order to ensure that we can read all
		// the data from connection, we use a for loop here.
		for {
			if !c.IsActive() {
				return tnet.ErrConnClosed
			}
			if err := handle(c); err != nil {
				return err
			}
		}
	})
}

// SetOnClosed sets the additional close process for a connection.
func (c *conn) SetOnClosed(handle Handler) error {
	return c.raw.SetOnClosed(func(_ tnet.Conn) error {
		return handle(c)
	})
}

// IsActive checks whether the connection is active or not.
func (c *conn) IsActive() bool {
	return c.raw.IsActive()
}
