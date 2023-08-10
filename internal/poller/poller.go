// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

// Package poller provides event driven polling system to monitor file description events.
package poller

import "fmt"

// Event defines the operation of poll.Control.
type Event int

// String implements fmt.Stringer.
func (e Event) String() string {
	switch e {
	case Readable:
		return "Readable"
	case ModReadable:
		return "ModReadable"
	case Writable:
		return "Writeable"
	case ModWritable:
		return "ModWriteable"
	case ReadWriteable:
		return "ReadWriteable"
	case ModReadWriteable:
		return "ModReadWriteable"
	case Detach:
		return "Detach"
	default:
		return fmt.Sprintf("Event(%d)", e)
	}
}

// Job function is defined for jobs.
type Job func() error

// Constants for PollEvents.
const (
	Readable Event = iota
	ModReadable
	Writable
	ModWritable
	ReadWriteable
	ModReadWriteable
	Detach
)

// Poller monitors file descriptor, calls Desc callbacks according to specific events.
type Poller interface {
	// Wait will poll all the registered Desc, and trigger the event callback
	// specified by the Desc
	Wait() error

	// Close closes the poller and stops Wait().
	Close() error

	// Trigger is used to trigger the poller to weak up from Wait(), each
	// Poller maintains a job queue, and does all the jobs after it wakes up.
	Trigger(Job) error

	// Control registers an event of Desc, which is defined by Event.
	Control(*Desc, Event) error
}
