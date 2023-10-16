// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

// Package buffer provides linked buffers.
package buffer

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/cache/systype"
	"trpc.group/trpc-go/tnet/internal/iovec"
)

const (
	// defaultBlockSize represents the default block size.
	defaultBlockSize = 4096
	// defaultMaxBufferSize represents the default max buffer size.
	defaultMaxBufferSize = 10 * 1024 * 1024
	// maxNodeBlockSize represents the maximum node block size.
	maxNodeBlockSize = 128 * 1024
)

var (
	// blockSize set the block size.
	blockSize = defaultBlockSize
	// maxFillLen max data can be filled to buffer once.
	maxFillLen = blockSize * systype.MaxLen
	// MaxBufferSize max buffer size to fill
	MaxBufferSize = defaultMaxBufferSize
	// cleanUp indicates whether to enable clean up feature for all buffers.
	cleanUp = false
)

var (
	// ErrNoEnoughData denotes that data in buffer is not enough than expected.
	ErrNoEnoughData = errors.New("buffer: buffer data is not enough")
	// ErrNodeFull denotes that not enough space is left in node for storing data.
	ErrNodeFull = errors.New("buffer: buffer's node isFull")
	// ErrInvalidParam denotes that param is invalid.
	ErrInvalidParam = errors.New("buffer: param is invalid")
	// ErrBufferFull denotes that buffer is full.
	ErrBufferFull = errors.New("buffer: buffer is full")
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return &Buffer{}
	},
}

// Buffer implements buffer chains to store data, it's safe to access data concurrently.
// The internal locker can make sure that only one reader and writer will be accessing data at any time.
type Buffer struct {
	head  *node
	tail  *node
	rnode *node
	wnode *node

	// rlock can make sure only one reader to reduce rlen.
	rlock sync.Mutex
	// wlock can make sure only one writer to modify wlen.
	wlock sync.Mutex

	rlen uint32
	wlen uint32

	// nodeBlockSize denotes the size of the block when adding
	// a new node to the buffer.
	nodeBlockSize uint32
	// enableAutoNodeBlockSize decides whether to auto adjust the node block size.
	enableAutoNodeBlockSize bool
}

// New allocates a buffer from buffer pool.
func New() *Buffer {
	b := bufferPool.Get().(*Buffer)
	b.Initialize()
	return b
}

// Initialize buffer before using it.
func (b *Buffer) Initialize() {
	b.rwLock()
	defer b.rwUnlock()
	dumpNode := newDumpNode()
	b.head, b.rnode, b.wnode, b.tail = dumpNode, dumpNode, dumpNode, dumpNode
	b.nodeBlockSize = uint32(blockSize)
	b.SetAutoNodeBlockSize(true)
}

// SetAutoNodeBlockSize enables or disables auto adjustment of the node block size.
func (b *Buffer) SetAutoNodeBlockSize(enable bool) {
	b.enableAutoNodeBlockSize = enable
}

// Free resets buffer and frees the nodes.
func (b *Buffer) Free() {
	b.rwLock()
	defer b.rwUnlock()
	for pNode := b.head; pNode != nil; {
		next := pNode.next
		freeNode(pNode)
		pNode = next
	}
	b.reset()
}

// Free resets buffer and puts it back to buffer pool.
func Free(b *Buffer) {
	b.Free()
	bufferPool.Put(b)
}

// SetCleanUp sets whether to enable clean up feature for all buffers.
// When clean up feature is enabled, the buffer will be set to init
// state when there is no more data in the buffer.
// It's used for 100w connections scenario to save memory.
func SetCleanUp(b bool) {
	cleanUp = b
}

// Peek returns the next n bytes without advancing the buffer.
// If the data length is less than n, return ErrNoEnoughData error.
// Zero-Copy API.
func (b *Buffer) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidParam
	}
	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() < n {
		return nil, ErrNoEnoughData
	}
	// find first non zero rnode
	for b.rnode.len() == 0 {
		b.rnode = b.rnode.next
	}
	rnode := b.rnode
	if rnode.len() >= n {
		return rnode.peek(n)
	}
	// not consistent space, allocate a new space and copy data together
	res := make([]byte, n)
	var ack int
	for ack < n && rnode != nil {
		if rnode.len() == 0 {
			rnode = rnode.next
			continue
		}
		offset := rnode.len()
		if ack+offset > n {
			offset = n - ack
		}
		s, err := rnode.peek(offset)
		if err != nil {
			return nil, err
		}
		copy(res[ack:ack+offset], s)
		ack += offset
		rnode = rnode.next
	}
	res = res[:ack]
	return res, nil
}

// Skip the next n bytes and advance the buffer, if the data length is less than
// n, return ErrNoEnoughData error.
func (b *Buffer) Skip(n int) error {
	if n < 0 {
		return ErrInvalidParam
	}
	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() < n {
		return errors.Wrap(ErrNoEnoughData, fmt.Sprintf("b.LenRead() = %d, want %d", b.LenRead(), n))
	}

	if b.rnode.len() >= n {
		if err := b.rnode.skip(n); err != nil {
			return err
		}
		atomic.AddUint32(&b.rlen, ^uint32(n-1))
		return nil
	}

	rnode := b.rnode
	var ack int
	for ack < n && rnode != nil {
		if rnode.len() == 0 {
			rnode = rnode.next
			continue
		}
		offset := rnode.len()
		if ack+offset > n {
			offset = n - ack
		}
		if err := rnode.skip(offset); err != nil {
			return err
		}
		ack += offset
	}
	b.rnode = rnode
	atomic.AddUint32(&b.rlen, ^uint32(ack-1))
	return nil
}

// Next returns the next n bytes and advances the buffer.
// If the data length is less than n, return ErrNoEnoughData error.
// Zero-Copy API.
func (b *Buffer) Next(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrInvalidParam
	}
	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() < n {
		return nil, ErrNoEnoughData
	}
	// find first non zero rnode
	for b.rnode.len() == 0 {
		b.rnode = b.rnode.next
	}
	rnode := b.rnode
	if rnode.len() >= n {
		s, err := rnode.readn(n)
		if err == nil {
			atomic.AddUint32(&b.rlen, ^uint32(n-1))
		}
		return s, err
	}
	// not consistent space, allocate a new space and copy data together
	res := make([]byte, n)
	var ack int
	for ack < n && rnode != nil {
		if rnode.len() == 0 {
			rnode = rnode.next
			continue
		}
		offset := rnode.len()
		if ack+offset > n {
			offset = n - ack
		}
		s, err := rnode.readn(offset)
		if err != nil {
			return nil, err
		}
		copy(res[ack:ack+offset], s)
		ack += offset
	}
	b.rnode = rnode
	res = res[:ack]
	atomic.AddUint32(&b.rlen, ^uint32(ack-1))
	return res, nil
}

// Read copies data from buffer and advances the pointer, it will release unused buffer automatically.
func (b *Buffer) Read(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}
	defer b.Release()

	b.rlock.Lock()
	defer b.rlock.Unlock()
	rlen := b.LenRead()
	if rlen < n {
		n = rlen
	}
	if b.rnode.len() >= n {
		s, err := b.rnode.readn(n)
		if err != nil {
			return 0, err
		}
		atomic.AddUint32(&b.rlen, ^uint32(n-1))
		copy(p, s)
		return n, nil
	}
	rnode := b.rnode
	var ack int
	for ack < n && rnode != nil {
		if rnode.len() == 0 {
			rnode = rnode.next
			continue
		}
		offset := rnode.len()
		if ack+offset > n {
			offset = n - ack
		}
		s, err := rnode.readn(offset)
		if err != nil {
			return 0, err
		}
		copy(p[ack:ack+offset], s)
		ack += offset
	}
	b.rnode = rnode
	atomic.AddUint32(&b.rlen, ^uint32(ack-1))
	return n, nil
}

// PeekBlocks peeks blocks from buffer as many as possible, and returns the block numbers it gets.
// This will not advance the buffer. In general, it should be used with Skip(n).
func (b *Buffer) PeekBlocks(data [][]byte) int {
	if len(data) == 0 {
		return 0
	}

	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() == 0 {
		return 0
	}
	var blocks int
	for rest, rnode := b.LenRead(), b.rnode; blocks < len(data) && rest > 0; rnode = rnode.next {
		l := rnode.len()
		if l != 0 {
			if l > rest {
				l = rest
			}
			data[blocks] = rnode.block[rnode.r : int(rnode.r)+l]
			rest -= l
			blocks++
		}
	}
	return blocks
}

// ReadBlock reads one block from buffer(reject 0 len block), and advances the buffer.
func (b *Buffer) ReadBlock() ([]byte, error) {
	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() <= 0 {
		return nil, ErrNoEnoughData
	}
	for b.rnode.len() == 0 {
		b.rnode = b.rnode.next
	}
	l := b.rnode.len()
	s, err := b.rnode.readn(l)
	if err == nil {
		atomic.AddUint32(&b.rlen, ^uint32(l-1))
	}
	return s, err
}

// SkipBlocks skips n blocks(except 0 len block) from buffer and advances the buffer.
func (b *Buffer) SkipBlocks(n int) error {
	if n < 0 {
		return ErrInvalidParam
	}
	b.rlock.Lock()
	defer b.rlock.Unlock()
	if b.LenRead() <= 0 {
		return ErrNoEnoughData
	}
	rnode := b.rnode
	var ack, ackn int
	for ackn < n && rnode != nil {
		if rnode.len() == 0 {
			rnode = rnode.next
			continue
		}
		l := rnode.len()
		if err := rnode.skip(l); err != nil {
			return err
		}
		ackn++
		ack += l
	}
	if ackn != n {
		return ErrNoEnoughData
	}
	b.rnode = rnode
	atomic.AddUint32(&b.rlen, ^uint32(ack-1))
	return nil
}

// OptimizeMemory further reduces buffer memory usage when the buffer is empty.
// If cleanUp is true, recycle all the unused blocks.
func (b *Buffer) OptimizeMemory() {
	b.rwLock()
	defer b.rwUnlock()
	if b.LenRead() != 0 {
		return
	}
	if cleanUp {
		b.CleanUpWithLock()
		return
	}
	atomic.AddUint32(&b.wlen, uint32(b.wnode.w))
	// If b.LenRead() == 0, rnode and wnode must be pointing to the same node.
	// Reset write node read/write pointer(aka. offset)
	// to avoid massive node get(put) operation from(to) the pool.
	b.wnode.r, b.wnode.w = 0, 0
}

// CleanUpWithLock do the actual clean up logic with rwlock on.
func (b *Buffer) CleanUpWithLock() {
	for pNode := b.head; pNode != nil; {
		next := pNode.next
		freeNode(pNode)
		pNode = next
	}
	dumpNode := newDumpNode()
	b.head, b.rnode, b.wnode, b.tail = dumpNode, dumpNode, dumpNode, dumpNode
	atomic.StoreUint32(&b.wlen, 0)
	// Reset node block size.
	b.nodeBlockSize = uint32(blockSize)
}

type chain struct {
	head    *node
	tail    *node
	dataLen int
}

func (c *chain) initialize(recycle bool, bs ...[]byte) {
	var prev *node
	for _, p := range bs {
		if len(p) == 0 {
			continue
		}
		n := allocNode()
		n.recycle = recycle
		n.setBlock(p)
		if c.head == nil {
			c.head = n
		}
		c.tail = n
		if prev != nil {
			prev.next = n
		}
		prev = n
		c.dataLen += len(p)
	}
}

// Write appends data to buffer.
//
// If safeWrite = true, p will be copied into the buffer.
// If safeWrite = false, p will be linked into the buffer(holds a reference to the buffer).
func (b *Buffer) Write(safeWrite bool, p []byte) int {
	return b.Writev(safeWrite, p)
}

// Writev appends data slice to buffer in order.
//
// If safeWrite = true, p will be copied into the buffer.
// If safeWrite = false, p will be linked into the buffer(holds a reference to the buffer).
func (b *Buffer) Writev(safeWrite bool, bs ...[]byte) int {
	if safeWrite { // Safe write needs copy.
		return b.copyFrom(bs...)
	}
	return b.linkFrom(bs...)
}

func (b *Buffer) copyFrom(bs ...[]byte) int {
	b.wlock.Lock()
	defer b.wlock.Unlock()
	b.prepareFill()
	var length int
	for i := range bs {
		length += len(bs[i])
	}
	b.malloc(b.calNodes(length))
	var copied int
	for i := range bs {
		copied += b.copyFromSingleByteSlice(bs[i])
	}
	return copied
}

func (b *Buffer) copyFromSingleByteSlice(buf []byte) int {
	length := len(buf)
	var copied int
	for pNode := b.wnode; pNode != nil; pNode = pNode.next {
		copied += copy(pNode.block[pNode.w:], buf[copied:])
		if copied >= length {
			break
		}
	}
	b.adjust(copied)
	return copied
}

func (b *Buffer) linkFrom(bs ...[]byte) int {
	total := len(bs)
	b.wlock.Lock()
	defer b.wlock.Unlock()
	linked, length := b.linkWithExistingNodes(bs...)
	if linked < total {
		length += b.linkWithNewNodes(bs[linked:]...)
	}
	return length
}

func (b *Buffer) linkWithExistingNodes(bs ...[]byte) (linked int, length int) {
	cnt := len(bs)
	var wlen int
	for pNode := b.wnode; pNode != nil; pNode = pNode.next {
		if linked >= cnt {
			break
		}
		if pNode.w != 0 {
			continue
		}
		wlen += pNode.rest()
		pNode.setBlock(bs[linked])
		length += len(bs[linked])
		b.wnode = pNode
		linked++
	}
	atomic.AddUint32(&b.rlen, uint32(length))
	atomic.AddUint32(&b.wlen, ^uint32(wlen-1))
	return
}

func (b *Buffer) linkWithNewNodes(bs ...[]byte) int {
	var c chain
	c.initialize(false, bs...)
	if c.dataLen == 0 {
		return 0
	}
	b.addChain(&c)
	return c.dataLen
}

// Reader is used by Buffer to fill data from data source such as socket.
type Reader interface {
	Readv(ivs []unix.Iovec) (int, error)
}

// Fill reads data from reader and fill it to buffer.
//
// In general, there are 2 ways to write data to buffer:
//  1. call Write()/Writev(), which is used by user to write data.
//  2. call Fill(), which is used to combine a data source(such as socket) and buffer together,
//     and when reader data is ready, call Fill() to write it to buffer.
//
// Please don't mix these two methods for one buffer.
func (b *Buffer) Fill(r Reader, n int, ioData *iovec.IOData) error {
	if b.LenRead() > MaxBufferSize {
		return ErrBufferFull
	}
	b.wlock.Lock()
	defer b.wlock.Unlock()
	// make sure the buffer has free space
	b.prepareFill()
	// calculate how many blocks need be allocated to fill n byte data
	nodeNum := b.calNodes(n)
	// allocate blocks to buffer, and convert it to IOVS data struct
	b.malloc(nodeNum)
	sliceCnt := b.countNodeAndFillByteVec(ioData)
	ioData.SetIOVec(sliceCnt)
	defer ioData.Release(sliceCnt)
	// read data from reader, and fill to IOVS(blocks)
	actual, err := r.Readv(ioData.IOVec[:sliceCnt])
	if err != nil {
		return err
	}
	// adjust the wnode according to the actual received data.
	b.adjust(actual)
	return nil
}

func (b *Buffer) countNodeAndFillByteVec(ioData *iovec.IOData) int {
	for pNode := b.wnode; pNode != nil; pNode = pNode.next {
		ioData.ByteVec = append(ioData.ByteVec, pNode.block[pNode.w:])
	}
	return len(ioData.ByteVec)
}

func (b *Buffer) prepareFill() {
	if !b.wnode.isFull() {
		return
	}
	next := b.wnode.next
	if next != nil {
		b.wnode = next
		return
	}
	b.addNode()
	b.wnode = b.wnode.next
}

func (b *Buffer) calNodes(n int) int {
	reqLen := n
	if reqLen > maxFillLen {
		reqLen = maxFillLen
	}
	reqLen -= b.wnode.rest()
	if reqLen <= 0 {
		return 1
	}
	return b.calNodesNum(reqLen) + 1
}

func (b *Buffer) malloc(nodeNum int) {
	var existedN int
	for pNode := b.wnode; pNode != nil; pNode = pNode.next {
		existedN++
	}
	if existedN >= nodeNum {
		return
	}
	newN := nodeNum - existedN
	for i := 0; i < newN; i++ {
		b.addNode()
	}
}

func (b *Buffer) adjust(n int) {
	dataLen := n
	pNode := b.wnode
	for pNode != nil {
		rest := pNode.rest()
		if rest >= dataLen {
			pNode.w += uint32(dataLen)
			break
		}
		pNode.w += uint32(rest)
		dataLen -= rest
		pNode = pNode.next
	}
	b.wnode = pNode
	atomic.AddUint32(&b.wlen, ^uint32(n-1))
	atomic.AddUint32(&b.rlen, uint32(n))
}

// Release releases the blocks that is already read.
func (b *Buffer) Release() {
	b.rlock.Lock()
	// When Release() is called, the data on the previous read nodes
	// have already been skipped (i.e. node.r has been moved towards node.w)
	// therefore cap() must be used instead of len().
	readLength := b.rnode.cap()
	for b.head != b.rnode {
		h := b.head
		readLength += h.cap()
		b.head = b.head.next
		freeNode(h)
	}
	// Update NodeBlockSize as the maximum readLength
	// to minimize node allocation.
	if b.enableAutoNodeBlockSize && uint32(readLength) > b.nodeBlockSize && readLength < maxNodeBlockSize {
		for b.nodeBlockSize < uint32(readLength) {
			b.nodeBlockSize <<= 1
		}
	}
	b.rlock.Unlock()
	b.OptimizeMemory()
}

// LenRead returns how many data can be read in buffer.
func (b *Buffer) LenRead() int {
	l := atomic.LoadUint32(&b.rlen)
	return int(l)
}

// LenWrite returns how many data can be written to buffer.
func (b *Buffer) LenWrite() int {
	l := atomic.LoadUint32(&b.wlen)
	return int(l)
}

func (b *Buffer) addChain(c *chain) {
	if c == nil {
		return
	}
	b.wnode.next = c.head
	b.wnode = c.tail
	b.tail = c.tail
	atomic.AddUint32(&b.rlen, uint32(c.dataLen))
}

func (b *Buffer) addNode() {
	n := allocNodeBlock(int(b.nodeBlockSize))
	b.tail.next = n
	b.tail = n
	wlen := n.rest()
	atomic.AddUint32(&b.wlen, uint32(wlen))
	rlen := n.len()
	atomic.AddUint32(&b.rlen, uint32(rlen))
}

func (b *Buffer) reset() {
	b.rlen, b.wlen = 0, 0
	b.head, b.tail, b.rnode, b.wnode = nil, nil, nil, nil
}

func (b *Buffer) rwLock() {
	b.rlock.Lock()
	b.wlock.Lock()
}

func (b *Buffer) rwUnlock() {
	b.wlock.Unlock()
	b.rlock.Unlock()
}

func (b *Buffer) calNodesNum(n int) int {
	return (n + int(b.nodeBlockSize) - 1) / int(b.nodeBlockSize)
}

func newDumpNode() *node {
	n := allocNode()
	n.r, n.w = 0, 0
	return n
}
