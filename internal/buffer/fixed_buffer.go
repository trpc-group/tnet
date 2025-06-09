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

// Package buffer provides linked buffers and fixed buffer.
package buffer

import (
	"io"
	"sync"

	"go.uber.org/atomic"
)

// FixedReadBuffer is a fixed size buffer for reading.
// It's concurrent safe in Read, Peek, Skip, Next, ReadN.
type FixedReadBuffer struct {
	buf  []byte
	rlen atomic.Uint32
	pos  atomic.Uint32
	lock sync.Mutex
}

// Initialize initializes the buffer with the given buffer.
func (b *FixedReadBuffer) Initialize(buf []byte) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.buf = buf
	b.rlen.Store(uint32(len(buf)))
	b.pos.Store(0)
}

// Read copies data from buffer and advances the pointer, it will release unused buffer automatically.
func (b *FixedReadBuffer) Read(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	rlen := b.LenRead()
	if rlen == 0 {
		return 0, io.EOF
	}

	if rlen < n {
		n = rlen
	}

	curPos := b.CurPos()
	copy(p, b.buf[curPos:curPos+n])

	b.pos.Add(uint32(n))
	b.rlen.Sub(uint32(n))
	return n, nil
}

// Peek returns the next n bytes without advancing the buffer.
// If the data length is less than n, return EOF error.
// Zero-Copy API.
func (b *FixedReadBuffer) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidParam
	}
	if n == 0 {
		return []byte{}, nil
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	rlen := b.LenRead()
	if rlen < n {
		return nil, io.EOF
	}

	curPos := b.CurPos()
	return b.buf[curPos : curPos+n], nil
}

// Skip skips the next n bytes and advances the buffer.
// If the data length is less than n, return EOF error.
func (b *FixedReadBuffer) Skip(n int) error {
	if n < 0 {
		return ErrInvalidParam
	}

	if n == 0 {
		return nil
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	rlen := b.LenRead()
	if rlen < n {
		return io.EOF
	}

	b.pos.Add(uint32(n))
	b.rlen.Sub(uint32(n))
	return nil
}

// Next returns the next n bytes and advances the buffer.
// If the data length is less than n, return EOF error.
// Zero-Copy API.
func (b *FixedReadBuffer) Next(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidParam
	}

	if n == 0 {
		return []byte{}, nil
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	rlen := b.LenRead()
	if rlen < n {
		return nil, io.EOF
	}

	curPos := b.CurPos()
	b.pos.Add(uint32(n))
	b.rlen.Sub(uint32(n))
	return b.buf[curPos : curPos+n], nil
}

// ReadN reads fixed length of data from the buffer.
func (b *FixedReadBuffer) ReadN(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidParam
	}
	if n == 0 {
		return []byte{}, nil
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	rlen := b.LenRead()
	if rlen < n {
		return nil, io.EOF
	}

	curPos := b.CurPos()
	buf := make([]byte, n)
	copy(buf, b.buf[curPos:curPos+n])
	b.pos.Add(uint32(n))
	b.rlen.Sub(uint32(n))
	return buf, nil
}

// LenRead returns the length of the data in the buffer.
func (b *FixedReadBuffer) LenRead() int {
	return int(b.rlen.Load())
}

// CurPos returns the current position of the buffer.
func (b *FixedReadBuffer) CurPos() int {
	return int(b.pos.Load())
}
