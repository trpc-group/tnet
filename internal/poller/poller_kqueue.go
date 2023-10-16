// Tencent is pleased to support the open source community by making tnet available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tnet source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License can be found in the LICENSE file.

//go:build freebsd || dragonfly || darwin
// +build freebsd dragonfly darwin

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
)

const (
	defaultKevent = 64
)

type kqueue struct {
	fd              int
	notified        int32
	events          []unix.Kevent_t
	ioData          iovec.IOData
	ignoreTaskError bool
}

func newPoller(ignoreTaskError bool) (Poller, error) {
	kqueueFD, err := unix.Kqueue()
	if err != nil {
		return nil, os.NewSyscallError("kqueue", err)
	}
	// Provide FD_CLOEXEC flag for consistency with Go runtime.
	if _, err := unix.FcntlInt(uintptr(kqueueFD), unix.F_SETFD, unix.FD_CLOEXEC); err != nil {
		return nil, err
	}
	if _, err = unix.Kevent(kqueueFD, []unix.Kevent_t{{
		Ident:  0,
		Filter: unix.EVFILT_USER,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
	}}, nil, nil); err != nil {
		return nil, os.NewSyscallError("kevent add|clear", err)
	}
	poller := &kqueue{
		fd:              kqueueFD,
		events:          make([]unix.Kevent_t, defaultKevent),
		ioData:          iovec.NewIOData(),
		ignoreTaskError: ignoreTaskError,
	}
	// For poller's ioData, the length of the slice must be utilized to enable tcpOnRead's fill to read
	// as much data as possible. So here we need to reset the length to zero.
	poller.ioData.Reset()
	return poller, nil
}

// Close closes the poller and stops Wait().
func (k *kqueue) Close() error {
	return os.NewSyscallError("close", unix.Close(k.fd))
}

func (k *kqueue) notify() error {
	for {
		if _, err := unix.Kevent(k.fd, []unix.Kevent_t{{
			Ident:  0,
			Filter: unix.EVFILT_USER,
			Fflags: unix.NOTE_TRIGGER,
		}}, nil, nil); err != unix.EINTR && err != unix.EAGAIN {
			return os.NewSyscallError("kevent", err)
		}
	}
}

// Trigger is used to trigger the kqueue to weak up from Wait().
func (k *kqueue) Trigger(job Job) error {
	if atomic.CompareAndSwapInt32(&k.notified, 0, 1) {
		return k.notify()
	}
	return nil
}

func (k *kqueue) handle(n int) {
	var wakeUp bool
	var hups []*Desc

	for i := 0; i < n; i++ {
		event := k.events[i]
		if event.Ident == 0 {
			wakeUp = true
			continue
		}
		desc := *(**Desc)(unsafe.Pointer(&event.Udata))
		// The handler function may change at runtime, so for consistency,
		// we store them in a temporary variable.
		onRead, onWrite, data := desc.OnRead, desc.OnWrite, desc.Data
		// Read/Write and error events may be triggered at the same time,
		// so use if/else instead of switch/case to determine them separately.
		if event.Flags&unix.EV_EOF != 0 || event.Flags&unix.EV_ERROR != 0 {
			hups = append(hups, desc)
		}
		if event.Filter == unix.EVFILT_READ && event.Flags&unix.EV_ENABLE != 0 {
			if onRead != nil && data != nil {
				if err := onRead(data, &k.ioData); err != nil {
					if !k.ignoreTaskError {
						hups = append(hups, desc)
					}
				}
				// Reset length to be ready for next use.
				k.ioData.Reset()
			}
		}
		if event.Filter == unix.EVFILT_WRITE && event.Flags&unix.EV_ENABLE != 0 {
			if onWrite != nil && data != nil {
				if err := onWrite(data); err != nil {
					if !k.ignoreTaskError {
						hups = append(hups, desc)
					}
				}
			}
		}
		if GoschedAfterEvent {
			runtime.Gosched()
		}
	}

	if wakeUp {
		k.runAsyncTasks()
	}
	if len(hups) > 0 {
		k.detach(hups)
	}
}

func (k *kqueue) runAsyncTasks() {
	atomic.StoreInt32(&k.notified, 0)
}

func (k *kqueue) detach(hups []*Desc) {
	for i := range hups {
		k.Control(hups[i], Detach)
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

// Wait will poll all the registered Desc, and trigger the event callback
// specified by the Desc.
func (k *kqueue) Wait() error {
	var zeroTimespec unix.Timespec
	var timespec *unix.Timespec

	for {
		n, err := unix.Kevent(k.fd, nil, k.events, timespec)
		if n == 0 || (n < 0 && err == unix.EINTR) {
			timespec = nil
			runtime.Gosched()
			continue
		} else if err != nil {
			return err
		}
		timespec = &zeroTimespec
		k.handle(n)
	}
}

func (k *kqueue) addRead(desc *Desc, flags uint16) error {
	evt := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_RECEIPT | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt}, nil, nil)
	return err
}

func (k *kqueue) addWrite(desc *Desc, flags uint16) error {
	evt := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_WRITE,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_RECEIPT | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt}, nil, nil)
	return err
}

func (k *kqueue) addReadWrite(desc *Desc, flags uint16) error {
	evt1 := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_RECEIPT | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt1.Udata)) = desc
	evt2 := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_WRITE,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_RECEIPT | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt2.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt1, evt2}, nil, nil)
	return err
}

func (k *kqueue) modRead(desc *Desc, flags uint16) error {
	evt := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_WRITE,
		Flags:  unix.EV_DELETE | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt}, nil, nil)
	return err
}

func (k *kqueue) modWrite(desc *Desc, flags uint16) error {
	evt := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_DELETE | flags,
	}
	*(**Desc)(unsafe.Pointer(&evt.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt}, nil, nil)
	return err
}

func (k *kqueue) delete(desc *Desc) error {
	evt1 := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_DELETE,
	}
	*(**Desc)(unsafe.Pointer(&evt1.Udata)) = desc
	evt2 := unix.Kevent_t{
		Ident:  newKeventIdent(desc.FD),
		Filter: unix.EVFILT_WRITE,
		Flags:  unix.EV_DELETE,
	}
	*(**Desc)(unsafe.Pointer(&evt2.Udata)) = desc
	_, err := unix.Kevent(k.fd, []unix.Kevent_t{evt1, evt2}, nil, nil)
	return err
}

// Control registers an event of Desc, which is defined by Event.
func (k *kqueue) Control(desc *Desc, event Event) (err error) {
	defer func() {
		err = errors.Wrap(err, fmt.Sprintf("event: %s, connection may be closed", event))
	}()
	switch event {
	case Readable:
		return k.addRead(desc, 0)
	case ModReadable:
		return k.modRead(desc, 0)
	case Writable:
		return k.addWrite(desc, 0)
	case ModWritable:
		return k.modWrite(desc, 0)
	case ReadWriteable, ModReadWriteable:
		return k.addReadWrite(desc, 0)
	case Detach:
		return k.delete(desc)
	default:
		return errors.New("Event not support")
	}
}
