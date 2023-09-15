// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

//go:build linux
// +build linux

package poller

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/poller/event"
	"trpc.group/trpc-go/tnet/log"
	"trpc.group/trpc-go/tnet/metrics"
)

const (
	rflags            = unix.EPOLLIN | unix.EPOLLRDHUP | unix.EPOLLHUP | unix.EPOLLERR | unix.EPOLLPRI
	wflags            = unix.EPOLLOUT | unix.EPOLLHUP | unix.EPOLLERR
	defaultEventCount = 64
)

func newPoller(ignoreTaskError bool) (Poller, error) {
	// Provide EPOLL_CLOEXEC flag for consistency with Go runtime.
	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, os.NewSyscallError("epoll_create1", err)
	}
	// Provide EFD_CLOEXEC flag for consistency with Go runtime.
	efd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return nil, os.NewSyscallError("eventfd", err)
	}
	desc := alloc()
	desc.FD = efd
	poller := &epoll{
		fd:              fd,
		desc:            desc,
		events:          make([]event.EpollEvent, defaultEventCount),
		ioData:          iovec.NewIOData(),
		buf:             make([]byte, 8),
		ignoreTaskError: ignoreTaskError,
	}
	// For poller's ioData, the length of the slice must be utilized to enable tcpOnRead's fill to read
	// as much data as possible. So here we need to reset the length to zero.
	poller.ioData.Reset()
	poller.Control(poller.desc, Readable)
	return poller, nil
}

type epoll struct {
	desc            *Desc
	ioData          iovec.IOData
	buf             []byte
	events          []event.EpollEvent
	fd              int
	notified        int32
	ignoreTaskError bool
}

func epollWait(epfd int, events []event.EpollEvent, msec int) (n int, err error) {
	var r0 uintptr
	var _p0 = unsafe.Pointer(&events[0])
	if msec == 0 {
		r0, _, err = unix.RawSyscall6(unix.SYS_EPOLL_PWAIT,
			uintptr(epfd), uintptr(_p0), uintptr(len(events)), 0, 0, 0)
		metrics.Add(metrics.EpollNoWait, 1)
	} else {
		r0, _, err = unix.Syscall6(unix.SYS_EPOLL_PWAIT,
			uintptr(epfd), uintptr(_p0), uintptr(len(events)), uintptr(msec), 0, 0)
	}
	if err == unix.Errno(0) {
		err = nil
	}
	metrics.Add(metrics.EpollWait, 1)
	metrics.Add(metrics.EpollEvents, uint64(r0))
	return int(r0), err
}

// Wait will poll all the registered Desc, and trigger the event callback
// specified by the Desc.
func (ep *epoll) Wait() error {
	msec := -1
	for {
		n, err := epollWait(ep.fd, ep.events, msec)
		if err != nil && err != unix.EINTR {
			return err
		}
		if n <= 0 {
			msec = -1
			runtime.Gosched()
			continue
		}
		msec = 0
		ep.handle(n)
	}
}

func (ep *epoll) notify() error {
	for {
		if _, err := unix.Write(ep.desc.FD, ep.buf); err != unix.EINTR && err != unix.EAGAIN {
			return os.NewSyscallError("write", err)
		}
	}
}

func (ep *epoll) handle(n int) {
	var hups []*Desc
	var wakeUp bool
	for i := 0; i < n; i++ {
		event := ep.events[i]
		desc := *(**Desc)(unsafe.Pointer(&event.Data))
		if desc.FD == ep.desc.FD {
			_, _ = unix.Read(ep.desc.FD, ep.buf)
			wakeUp = true
			continue
		}
		// inHup guarantees that each descriptor will be appended to `hups` only once.
		var inHup bool
		// Read/Write and error events may be triggered at the same time,
		// so use if/else instead of switch/case to determine them separately.
		if event.Events&(unix.EPOLLHUP|unix.EPOLLRDHUP|unix.EPOLLERR) != 0 {
			inHup = true
		}
		readable := event.Events&(unix.EPOLLIN|unix.EPOLLPRI) != 0
		writable := event.Events&(unix.EPOLLOUT) != 0
		// The handler function may change at runtime, so for consistency,
		// we store them in a temporary variable.
		onRead, onWrite, data := desc.OnRead, desc.OnWrite, desc.Data
		if writable && onWrite != nil && data != nil {
			if err := onWrite(data); err != nil {
				log.Debugf("onWrite err: %v\n", err)
				if !ep.ignoreTaskError {
					inHup = true
				}
			}
		}
		if readable && onRead != nil && data != nil {
			if err := onRead(data, &ep.ioData); err != nil {
				log.Debugf("onRead err: %v\n", err)
				if !ep.ignoreTaskError {
					inHup = true
				}
			}
			// Reset length to be ready for next use.
			ep.ioData.Reset()
		}
		if inHup {
			hups = append(hups, desc)
		}
	}
	if wakeUp {
		ep.runAsyncTasks()
	}
	if len(hups) > 0 {
		ep.detach(hups)
	}
}

func (ep *epoll) runAsyncTasks() {
	atomic.StoreInt32(&ep.notified, 0)
}

func (ep *epoll) detach(hups []*Desc) {
	for i := range hups {
		ep.Control(hups[i], Detach)
	}

	for i := range hups {
		desc := hups[i]
		if desc == nil {
			continue
		}
		data, onHup := desc.Data, desc.OnHup
		if data == nil || onHup == nil {
			continue
		}
		go onHup(data)
	}
	freeDesc()
}

// Close closes the poller and stops Wait().
func (ep *epoll) Close() error {
	if err := os.NewSyscallError("close", unix.Close(ep.fd)); err != nil {
		return err
	}
	return os.NewSyscallError("close", unix.Close(ep.desc.FD))
}

// Trigger is used to trigger the epoll to weak up from Wait().
func (ep *epoll) Trigger(job Job) error {
	if atomic.CompareAndSwapInt32(&ep.notified, 0, 1) {
		return ep.notify()
	}
	return nil
}

// Control the event of Desc and the operations is defined by Event.
func (ep *epoll) Control(desc *Desc, e Event) (err error) {
	evt := &event.EpollEvent{}
	*(**Desc)(unsafe.Pointer(&evt.Data)) = desc
	defer func() {
		if err != nil { // Prevent unconditional execution of fmt.Sprintf.
			err = errors.Wrap(err, fmt.Sprintf("event: %s, connection may be closed", e))
		}
	}()
	switch e {
	case Readable:
		evt.Events = rflags
		return ep.insert(desc.FD, evt)
	case Writable:
		evt.Events = wflags
		return ep.insert(desc.FD, evt)
	case ReadWriteable:
		evt.Events = rflags | wflags
		return ep.insert(desc.FD, evt)
	case ModReadable:
		evt.Events = rflags
		return ep.interest(desc.FD, evt)
	case ModWritable:
		evt.Events = wflags
		return ep.interest(desc.FD, evt)
	case ModReadWriteable:
		evt.Events = rflags | wflags
		return ep.interest(desc.FD, evt)
	case Detach:
		return ep.remove(desc.FD)
	default:
		return errors.New("Event not support")
	}
}

func (ep *epoll) insert(fd int, event *event.EpollEvent) error {
	if err := epollCtl(ep.fd, unix.EPOLL_CTL_ADD, fd, event); err != nil {
		return os.NewSyscallError("epoll_ctl add", err)
	}
	return nil
}

func (ep *epoll) interest(fd int, event *event.EpollEvent) error {
	if err := epollCtl(ep.fd, unix.EPOLL_CTL_MOD, fd, event); err != nil {
		return os.NewSyscallError("epoll_ctl mod", err)
	}
	return nil
}

func (ep *epoll) remove(fd int) error {
	if err := epollCtl(ep.fd, unix.EPOLL_CTL_DEL, fd, nil); err != nil {
		return os.NewSyscallError("epoll_ctl del", err)
	}
	return nil
}

func epollCtl(epfd int, op int, fd int, event *event.EpollEvent) error {
	var err error
	_, _, err = unix.RawSyscall6(
		unix.SYS_EPOLL_CTL,
		uintptr(epfd),
		uintptr(op),
		uintptr(fd),
		uintptr(unsafe.Pointer(event)),
		0, 0)
	if err == unix.Errno(0) {
		err = nil
	}
	return err
}
