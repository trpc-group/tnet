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

// Package timer provides functions of timer.
package timer

import "time"

// Timer type represents a single event.
type Timer struct {
	deadline time.Time
	timer    *time.Timer
}

// New creates a timer which will expire at time t.
// Make sure to call Start() to start the timer.
func New(t time.Time) *Timer {
	return &Timer{
		deadline: t,
	}
}

// Start begins the timer.
func (t *Timer) Start() {
	if t.timer == nil {
		t.timer = time.NewTimer(time.Until(t.deadline))
	} else {
		if !t.timer.Stop() {
			select {
			case <-t.timer.C:
			default:
			}
		}
		t.timer.Reset(time.Until(t.deadline))
	}
}

// Stop ends the timer and resets it to no timeout state.
func (t *Timer) Stop() {
	if t.timer == nil {
		return
	}
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
	t.deadline = time.Time{}
}

// Wait returns the time.Timer channel.
func (t *Timer) Wait() <-chan time.Time {
	return t.timer.C
}

// Reset resets the Timer.
// Make sure to call Start() after Reset() to use the newly set time.
func (t *Timer) Reset(u time.Time) {
	t.deadline = u
}

// Expired returns whether the timer has expired.
func (t *Timer) Expired() bool {
	if !t.deadline.IsZero() && t.deadline.Before(time.Now()) {
		return true
	}
	return false
}

// IsZero returns whether the timer is in no timeout state.
func (t *Timer) IsZero() bool {
	return t.deadline.IsZero()
}
