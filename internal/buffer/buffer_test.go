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

package buffer

import (
	"errors"
	"io"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/iovec"
)

func newReader(s string) Reader {
	r := &tReader{
		Reader: strings.NewReader(s),
	}
	return r
}

type tReader struct {
	io.Reader
}

func (r *tReader) Readv(iovec []unix.Iovec) (int, error) {
	var dataLen int
	for i := range iovec {
		dataLen += int(iovec[i].Len)
	}
	p := make([]byte, dataLen)
	n, err := r.Read(p)
	// If less than n is read, p needs to be truncated.
	p = p[:n]
	if err != nil {
		return n, err
	}
	ack := 0
	var i int
	wp := 0
	rp := 0
	for ack < n {
		if i >= len(iovec) {
			break
		}
		nc := copy(unsafe.Slice(iovec[i].Base, iovec[i].Len)[wp:], p[rp:])
		wp += nc
		if wp == int(iovec[i].Len) {
			i++
			wp = 0
		}
		rp += nc
		ack += nc
	}
	return ack, nil
}

func Test_NewBuffer(t *testing.T) {
	b := New()
	defer Free(b)
	assert.NotNil(t, b)
	assert.NotNil(t, b.head)
	assert.Equal(t, b.head, b.rnode)
	assert.Equal(t, b.head, b.wnode)
}

func TestBuffer_Writev(t *testing.T) {
	// 无参数
	b := New()
	defer Free(b)
	n := b.Writev(true)
	assert.Zero(t, 0, n)

	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	n = b.Writev(false, s1, s2, s3)
	assert.Equal(t, len(s1)+len(s2)+len(s3), n)
}

func TestBuffer_Write(t *testing.T) {
	// 无参数
	b := New()
	defer Free(b)
	s1 := []byte{1, 2, 3}
	n := b.Write(true, s1)
	assert.Equal(t, len(s1), n)
}

func TestBuffer_Peek(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2 := []byte{1, 2, 3}, []byte{4, 5, 6}
	b.Writev(false, s1, s2)

	_, err := b.Peek(-1)
	assert.Equal(t, err, ErrInvalidParam)

	// single node
	res, err := b.Peek(3)
	assert.Nil(t, err)
	assert.Equal(t, []byte{1, 2, 3}, res)

	res, err = b.Peek(5)
	assert.Nil(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4, 5}, res)

	// multiple nodes
	res, err = b.Peek(6)
	assert.Nil(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6}, res)

	_, err = b.Peek(7)
	assert.Equal(t, err, ErrNoEnoughData)
}

func TestBuffer_Peek_err(t *testing.T) {
	// 无参数
	b := New()
	defer Free(b)
	s1 := []byte{1, 2, 3}
	b.Writev(false, s1)

	res, err := b.Peek(4)
	assert.NotNil(t, err)
	assert.Nil(t, res)
}

func TestBuffer_Skip(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)

	assert.Equal(t, ErrInvalidParam, b.Skip(-1))

	// single node
	err := b.Skip(3)
	assert.Nil(t, err)
	assert.Equal(t, uint32(6), b.rlen.Load())

	assert.Nil(t, b.Skip(5))

	// single node
	err = b.Skip(1)
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), b.rlen.Load())

	assert.True(t, errors.Is(b.Skip(1), ErrNoEnoughData))
}

func TestBuffer_Skip_err(t *testing.T) {
	// 无参数
	b := New()
	defer Free(b)
	s1 := []byte{1, 2, 3}
	b.Writev(false, s1)

	err := b.Skip(4)
	assert.NotNil(t, err)
}

func TestBuffer_Next(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)

	_, err := b.Next(-1)
	assert.Equal(t, ErrInvalidParam, err)

	// single node
	res, err := b.Next(3)
	assert.Nil(t, err)
	assert.Equal(t, []byte{1, 2, 3}, res)

	// multiple nodes
	res, err = b.Next(6)
	assert.Nil(t, err)
	assert.Equal(t, []byte{4, 5, 6, 7, 8, 9}, res)
}

func TestBuffer_Next_err(t *testing.T) {
	// 无参数
	b := New()
	defer Free(b)
	s1 := []byte{1, 2, 3}
	b.Writev(false, s1)

	res, err := b.Next(4)
	assert.NotNil(t, err)
	assert.Nil(t, res)
}

func TestBuffer_Read(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)

	res := make([]byte, 0)
	_, err := b.Read(res)
	assert.Nil(t, err)

	res = make([]byte, 1)
	n, err := b.Read(res)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{1}, res)

	// single node
	res = make([]byte, 2)
	n, err = b.Read(res)
	assert.Nil(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte{2, 3}, res)

	// multiple nodes
	res = make([]byte, 4)
	n, err = b.Read(res)
	assert.Nil(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte{4, 5, 6, 7}, res)
}

func TestBuffer_Read_lessThanRequire(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)

	res := make([]byte, 10)
	n, err := b.Read(res)
	assert.Nil(t, err)
	assert.Equal(t, 9, n)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}, res[:n])
}

func TestBuffer_PeekBlocks(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)
	data := make([][]byte, 3)
	n := b.PeekBlocks(data)
	assert.Equal(t, 3, n)
	assert.Equal(t, s1, data[0])
	assert.Equal(t, s2, data[1])
	assert.Equal(t, s3, data[2])
}

func TestBuffer_ReadBlock(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)
	res, err := b.ReadBlock()
	assert.Nil(t, err)
	assert.Equal(t, s1, res)
	res, err = b.ReadBlock()
	assert.Nil(t, err)
	assert.Equal(t, s2, res)
	res, err = b.ReadBlock()
	assert.Nil(t, err)
	assert.Equal(t, s3, res)
}

func TestBuffer_SkipBlocks(t *testing.T) {
	b := New()
	defer Free(b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)
	err := b.SkipBlocks(1)
	assert.Nil(t, err)
	assert.Equal(t, 6, b.LenRead())
	err = b.SkipBlocks(2)
	assert.Nil(t, err)
	assert.Equal(t, 0, b.LenRead())
}

func testDump(t *testing.T, b *Buffer) {
	assert.Equal(t, b.head, b.rnode)
	assert.Equal(t, b.head, b.wnode)
	assert.Equal(t, b.head, b.tail)
	assert.Equal(t, 0, b.head.cap())
	assert.Equal(t, 0, cap(b.head.block))
}

func TestBuffer_CleanUp(t *testing.T) {
	b := New()
	defer Free(b)
	b.CleanUpWithLock()
	testDump(t, b)
	s1, s2, s3 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	b.Writev(false, s1, s2, s3)
	b.Next(len(s1) + len(s2) + len(s3))
	b.CleanUpWithLock()
	testDump(t, b)

	r := newReader("12345")
	ioData := iovec.NewIOData()
	ioData.Reset()
	b.Fill(r, 5, &ioData)
	b.Next(5)
	b.CleanUpWithLock()
	testDump(t, b)
}

func TestBuffer_Release(t *testing.T) {
	b := New()
	defer Free(b)
	SetCleanUp(true)
	s1 := "123456789123"
	saveNodeSize := blockSize
	defer func() {
		blockSize = saveNodeSize
	}()
	blockSize = 3
	r := newReader(s1)
	ioData := iovec.NewIOData()
	ioData.Reset()
	err := b.Fill(r, len(s1), &ioData)
	assert.Equal(t, uint32(len(s1)), b.rlen.Load())
	assert.Nil(t, err)

	// 单个 Node 的内容的话，不是拷贝的
	copyHead := b.head
	res, err := b.Next(3)
	b.Next(3)
	assert.Nil(t, err)
	assert.Equal(t, s1[:3], string(res))
	copyRes := make([]byte, 3)
	copy(copyRes, res)
	b.Release()
	assert.NotEqual(t, copyHead, b.head)

	// 多个 Node 的内容的话，是拷贝的
	res, err = b.Next(4)
	assert.Nil(t, err)
	assert.Equal(t, s1[6:10], string(res))
	copyRes = make([]byte, 4)
	copy(copyRes, res)
	b.Release()
	assert.Equal(t, copyRes, res)
}

func TestBuffer_Fill_smallBlockSize(t *testing.T) {
	s := "0123456789a1b2c3d4e5f6g7h8i9j1k2l3m4n5o6p7q8"
	r := newReader(s)
	saveNodeSize := blockSize
	defer func() {
		blockSize = saveNodeSize
	}()
	blockSize = 10
	b := New()
	defer Free(b)
	ioData := iovec.NewIOData()
	ioData.Reset()
	err := b.Fill(r, 10, &ioData)
	assert.Nil(t, err)
	p, err := b.Next(8)
	assert.Nil(t, err)
	assert.Equal(t, s[:8], string(p))
	p, err = b.Next(2)
	assert.Nil(t, err)
	assert.Equal(t, s[8:10], string(p))

	ioData = iovec.NewIOData()
	ioData.Reset()
	err = b.Fill(r, 10, &ioData)
	assert.Nil(t, err)
	p, err = b.Next(8)
	assert.Nil(t, err)
	assert.Equal(t, s[10:18], string(p))
	p, err = b.Next(2)
	assert.Nil(t, err)
	assert.Equal(t, s[18:20], string(p))
}

func TestBuffer_Fill_MaxBufferSize(t *testing.T) {
	s := "0123456789a1b2c3d4e5f6g7h8i9j1k2l3m4n5o6p7q8"
	r := newReader(s)
	saveNodeSize := blockSize
	defer func() {
		blockSize = saveNodeSize
	}()
	blockSize = 10
	b := New()
	defer Free(b)
	ioData := iovec.NewIOData()
	ioData.Reset()
	err := b.Fill(r, 10, &ioData)
	assert.Nil(t, err)

	MaxBufferSize = 8
	defer func() {
		MaxBufferSize = defaultMaxBufferSize
	}()
	ioData = iovec.NewIOData()
	ioData.Reset()
	err = b.Fill(r, 10, &ioData)
	assert.Equal(t, ErrBufferFull, err)
}

func TestBuffer_Fill_bufferNotFull(t *testing.T) {
	s := "012345"
	r := newReader(s)
	saveNodeSize := blockSize
	defer func() {
		blockSize = saveNodeSize
	}()
	blockSize = 10
	b := New()
	defer Free(b)
	ioData := iovec.NewIOData()
	ioData.Reset()
	err := b.Fill(r, len(s), &ioData)
	assert.Nil(t, err)
	p, err := b.Next(5)
	assert.Nil(t, err)
	assert.Equal(t, s[:5], string(p))
	p, err = b.Next(2)
	assert.NotNil(t, err)
	assert.Nil(t, p)

	r2 := newReader(s)
	ioData = iovec.NewIOData()
	ioData.Reset()
	err = b.Fill(r2, 10, &ioData)
	assert.Nil(t, err)
	p, err = b.Next(5)
	assert.Nil(t, err)
	assert.Equal(t, "50123", string(p))
}

func TestBuffer_Fill_moreThanOneNode(t *testing.T) {
	s := "123456"
	r := newReader(s)
	saveNodeSize := blockSize
	defer func() {
		blockSize = saveNodeSize
	}()
	blockSize = 10
	b := New()
	defer Free(b)
	ioData := iovec.NewIOData()
	ioData.Reset()
	err := b.Fill(r, 30, &ioData)
	assert.Nil(t, err)
	p, err := b.Next(5)
	assert.Nil(t, err)
	assert.Equal(t, s[:5], string(p))
	p, err = b.Next(2)
	assert.NotNil(t, err)
	assert.Nil(t, p)

	r2 := newReader(s)
	ioData = iovec.NewIOData()
	ioData.Reset()
	err = b.Fill(r2, 30, &ioData)
	assert.Nil(t, err)
	p, err = b.Next(5)
	assert.Nil(t, err)
	assert.Equal(t, "61234", string(p))
	p, err = b.Next(2)
	assert.Nil(t, err)
	assert.Equal(t, "56", string(p))
}

func TestBuffer_reset(t *testing.T) {
	b := New()
	defer Free(b)
	b.rnode = &node{}
	b.wnode = &node{}
	b.head = &node{}
	b.rlen.Store(10)
	b.reset()
	assert.Nil(t, b.head)
	assert.Nil(t, b.rnode)
	assert.Nil(t, b.wnode)
	assert.Zero(t, b.rlen.Load())
}

func Test_calNodeNum(t *testing.T) {
	b := &Buffer{nodeBlockSize: 6}
	n := b.calNodesNum(0)
	assert.Equal(t, 0, n)
	n = b.calNodesNum(5)
	assert.Equal(t, 1, n)
	n = b.calNodesNum(6)
	assert.Equal(t, 1, n)
	n = b.calNodesNum(7)
	assert.Equal(t, 2, n)
}
