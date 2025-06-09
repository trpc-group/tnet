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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newNode(t *testing.T) {
	n := allocNode()
	defer freeNode(n)
	assert.Nil(t, n.block)
	assert.Zero(t, int(n.r))
	assert.Zero(t, n.w)
	assert.Nil(t, n.next)
}

func Test_freeNode(t *testing.T) {
	var n *node
	freeNode(n)
	assert.Nil(t, n)

	n = allocNode()
	n.allocBlock()
	freeNode(n)
	assert.Nil(t, n.block)
}

func Test_node_alloc(t *testing.T) {
	n := allocNode()
	defer freeNode(n)
	n.allocBlock()
	assert.Equal(t, blockSize, n.cap())
	assert.Zero(t, n.len())
	assert.True(t, n.recycle)
}

func Test_node_allocN(t *testing.T) {
	n := allocNode()
	defer freeNode(n)
	size := 5
	n.allocBlockN(size)
	assert.Equal(t, size, n.cap())
	assert.Zero(t, n.len())
	assert.True(t, n.recycle)
}

func Test_node_len(t *testing.T) {
	s1 := []byte{1, 2, 3}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s1)
	length := n.len()
	assert.Equal(t, len(s1), length)
}

func Test_node_cap(t *testing.T) {
	s1 := []byte{1, 2, 3}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s1)
	assert.Equal(t, len(s1), n.cap())
}

func Test_node_full(t *testing.T) {
	s1 := []byte{1, 2, 3}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s1)
	isFull := n.isFull()
	assert.True(t, isFull)
}

func Test_node_peek(t *testing.T) {
	s := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s)

	// Data not enough error.
	p, err := n.peek(len(s) + 1)
	assert.Nil(t, p)
	assert.Equal(t, ErrNoEnoughData, err)

	// Read for the first time.
	p, err = n.peek(4)
	assert.Nil(t, err)
	assert.Equal(t, s[:4], p)
	assert.Equal(t, 0, int(n.r))

	// Read for the second time.
	p, err = n.peek(5)
	assert.Nil(t, err)
	assert.Equal(t, s[:5], p)
	assert.Equal(t, 0, int(n.r))
}

func Test_node_readn(t *testing.T) {
	s := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s)

	// Data not enough error.
	p, err := n.readn(len(s) + 1)
	assert.Nil(t, p)
	assert.Equal(t, ErrNoEnoughData, err)

	// Read for the first time.
	num1 := 4
	p, err = n.readn(num1)
	assert.Nil(t, err)
	assert.Equal(t, s[:num1], p)
	assert.Equal(t, num1, int(n.r))

	// Read for the second time.
	num2 := 5
	p, err = n.readn(num2)
	assert.Nil(t, err)
	assert.Equal(t, s[num1:num1+num2], p)
	assert.Equal(t, num1+num2, int(n.r))

	// Data not enough error.
	last := len(s) - num1 - num2
	p, err = n.readn(last + 1)
	assert.Nil(t, p)
	assert.Equal(t, ErrNoEnoughData, err)
}

func Test_node_skip(t *testing.T) {
	s := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	n := allocNode()
	defer freeNode(n)
	n.setBlock(s)

	// Data not enough error.
	err := n.skip(len(s) + 1)
	assert.Equal(t, ErrNoEnoughData, err)

	// Skip for the first time.
	num1 := 4
	err = n.skip(num1)
	assert.Nil(t, err)
	assert.Equal(t, num1, int(n.r))

	// Skip for the second time.
	num2 := 5
	err = n.skip(num2)
	assert.Nil(t, err)
	assert.Equal(t, num1+num2, int(n.r))

	// Data not enough error.
	last := len(s) - num1 - num2
	err = n.skip(last + 1)
	assert.Equal(t, ErrNoEnoughData, err)
}

func Test_node_add(t *testing.T) {
	n := allocNode()
	defer freeNode(n)
	size := 10
	p1 := []byte{1, 2, 3, 4, 5, 6}
	n.allocBlockN(size)
	num := copy(n.block, p1)
	err := n.add(num)
	assert.Nil(t, err)
	assert.Equal(t, len(p1), n.len())

	p2 := []byte{1, 2, 3, 4, 5}
	num = copy(n.block[n.w:], p2)
	// The buffer capacity exceeds when adjusting the write pointer.
	err = n.add(num + 1)
	assert.Equal(t, ErrNodeFull, err)
	// Buffer full when adjusting write pointer.
	err = n.add(num)
	assert.Nil(t, err)
	err = n.add(num)
	assert.Equal(t, ErrNodeFull, err)
}

func Test_node_setBlock(t *testing.T) {
	s1 := []byte{1, 2, 3}
	n := allocNode()
	defer freeNode(n)
	np := allocNode()
	defer freeNode(np)
	nn := allocNode()
	defer freeNode(nn)
	n.setBlock(s1)
	isSameUnderlayer := reflect.ValueOf(s1).Pointer() == reflect.ValueOf(n.block).Pointer()
	assert.True(t, isSameUnderlayer)
}

func Test_node_reset(t *testing.T) {
	// Recyclable node.
	n := allocNode()
	defer freeNode(n)
	n.allocBlock()
	n.reset()
	assert.Nil(t, n.block)
	assert.Zero(t, int(n.r))
	assert.Zero(t, n.w)
	assert.Nil(t, n.next)

	// Not recyclable node.
	n = allocNode()
	defer freeNode(n)
	n1 := allocNode()
	defer freeNode(n1)
	n2 := allocNode()
	defer freeNode(n2)
	p := []byte{1, 2, 3}
	n.setBlock(p)
	n.reset()
	assert.Nil(t, n.block)
	assert.Zero(t, int(n.r))
	assert.Zero(t, n.w)
	assert.Nil(t, n.next)
}
