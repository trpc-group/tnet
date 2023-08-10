// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package systype provides system type such as unix.Ioves.
// Reuses [][]byte and []MMsghdr.
package systype

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// MaxLen is the maximum length for []unix.Iovec, [][]byte, []MMsghdr.
	MaxLen = 64
)

// IOVECWrapper is a wrapper for []unix.Iovec struct.
type IOVECWrapper struct {
	iovec []unix.Iovec
}

var iovecPool sync.Pool = sync.Pool{
	New: func() interface{} {
		return &IOVECWrapper{
			iovec: make([]unix.Iovec, 0, MaxLen),
		}
	},
}

// GetIOVECWrapper gets a []unix.Iovec with fixed capacity of length len(bs).
// Release it using PutIOVECWrapper.
func GetIOVECWrapper(bs [][]byte) ([]unix.Iovec, *IOVECWrapper) {
	var (
		v []unix.Iovec
		h *IOVECWrapper
	)
	if len(bs) <= MaxLen {
		h = iovecPool.Get().(*IOVECWrapper)
		v = h.iovec
	} else {
		v = make([]unix.Iovec, 0, len(bs))
	}

	for _, b := range bs {
		if len(b) == 0 {
			continue
		}
		v = append(v, unix.Iovec{
			Base: &b[0],
			Len:  converUint(len(b)),
		})
	}
	return v, h
}

// PutIOVECWrapper release a []unix.Iovec.
func PutIOVECWrapper(h *IOVECWrapper) {
	if cap(h.iovec) != MaxLen {
		return
	}
	h.iovec = h.iovec[:0]
	iovecPool.Put(h)
}

// IODatas is a wrapper for [][]byte struct.
type IODatas struct {
	D [][]byte
}

var ioDataPool sync.Pool = sync.Pool{
	New: func() interface{} {
		return &IODatas{
			D: make([][]byte, 0, MaxLen),
		}
	},
}

// GetIODatas get a [][]byte with fixed capacity.
// Release it using PutIODatas.
func GetIODatas(size int) ([][]byte, *IODatas) {
	if size > MaxLen {
		return make([][]byte, size), nil
	}
	d := ioDataPool.Get().(*IODatas)
	return d.D[:size], d
}

// PutIODatas release a [][]byte.
func PutIODatas(d *IODatas) {
	if cap(d.D) != MaxLen {
		return
	}
	d.D = d.D[:0]
	ioDataPool.Put(d)
}

//-------------------------------------MMsghdr------------------------------------

// MMsghdr is the input parameter of recvmmsg.
type MMsghdr struct {
	Hdr unix.Msghdr
	Len uint32
	_   [4]byte // Pad with 4 bytes to align 64bit machine.
}

var (
	// mmsghdrs => []MMsghdr
	mmsghdrsPool sync.Pool
)

func init() {
	mmsghdrsPool.New = func() interface{} {
		mmsghdrs := make([]MMsghdr, MaxLen)
		return mmsghdrs
	}
}

// GetMMsghdrs gets a []mmsghdr with fixed capacity.
// Release it with PutMMsghdrs.
func GetMMsghdrs(size int) []MMsghdr {
	if size > MaxLen {
		return make([]MMsghdr, size)
	}
	mmsghdrs := mmsghdrsPool.Get().([]MMsghdr)
	mmsghdrs = mmsghdrs[:size]
	return mmsghdrs
}

// BuildMMsg fills MMsghdr with name and buffer.
func BuildMMsg(m *MMsghdr, name, buf []byte) {
	m.Hdr.Iov = &unix.Iovec{}
	m.Hdr.Iovlen = 1
	m.Hdr.Iov.Base = (*byte)(unsafe.Pointer(&buf[0]))
	m.Hdr.Iov.Len = converUint(len(buf))
	m.Hdr.Name = (*byte)(unsafe.Pointer(&name[0]))
	m.Hdr.Namelen = uint32(len(name))
}

// PutMMsghdrs release a []mmsghdr.
func PutMMsghdrs(mmsghdrs []MMsghdr) {
	if cap(mmsghdrs) != MaxLen {
		return
	}
	mmsghdrsPool.Put(mmsghdrs[:0])
}
