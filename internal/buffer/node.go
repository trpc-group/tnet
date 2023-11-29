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
	"sync"

	"trpc.group/trpc-go/tnet/internal/cache/mcache"
)

var nodePool = sync.Pool{
	New: func() any {
		return &node{}
	},
}

// node contains a continuous byte slice memory space.
type node struct {
	next    *node
	block   []byte
	r       uint32
	w       uint32
	recycle bool
}

// allocNode gets a node from pool. The node has not yet
// been allocated byte slice memory.
func allocNode() *node {
	return nodePool.Get().(*node)
}

func allocNodeBlock(size int) *node {
	n := allocNode()
	n.allocBlockN(size)
	return n
}

func freeNode(n *node) {
	if n == nil {
		return
	}
	n.reset()
	nodePool.Put(n)
}

func (n *node) allocBlock() {
	n.allocBlockN(blockSize)
}

func (n *node) allocBlockN(size int) {
	n.block = mcache.Malloc(size)
	n.recycle = true
}

func (n *node) len() int {
	return int(n.w - n.r)
}

func (n *node) rest() int {
	return n.cap() - int(n.w)
}

func (n *node) cap() int {
	return len(n.block)
}

func (n *node) isFull() bool {
	return (int(n.w) == len(n.block))
}

func (n *node) peek(num int) ([]byte, error) {
	if num > n.len() {
		return nil, ErrNoEnoughData
	}
	p := n.block[n.r : int(n.r)+num]
	return p, nil
}

func (n *node) readn(num int) ([]byte, error) {
	if num > n.len() {
		return nil, ErrNoEnoughData
	}
	p := n.block[n.r : int(n.r)+num]
	n.r = n.r + uint32(num)
	return p, nil
}

func (n *node) skip(num int) error {
	if num > n.len() {
		return ErrNoEnoughData
	}
	n.r = n.r + uint32(num)
	return nil
}

func (n *node) add(wlen int) error {
	if n.isFull() {
		return ErrNodeFull
	}
	if int(n.w)+wlen > len(n.block) {
		return ErrNodeFull
	}
	n.w = n.w + uint32(wlen)
	return nil
}

func (n *node) setBlock(b []byte) {
	// Before setting up a new block, if the node contains an old
	// block that can be recycled, it needs to be recycled first.
	if n.recycle && n.block != nil {
		mcache.Free(n.block)
	}
	n.block = b
	n.w = uint32(len(b))
	n.recycle = false
}

func (n *node) reset() {
	if n.recycle {
		mcache.Free(n.block)
	}
	n.block = nil
	n.next = nil
	n.recycle = false
	n.r = 0
	n.w = 0
}
