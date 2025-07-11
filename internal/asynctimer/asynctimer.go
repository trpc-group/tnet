//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 Tencent.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

// Package asynctimer provides asynchronous timer function, which is implemented by time wheel.
package asynctimer

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/atomic"
)

const (
	defaultInterval = time.Second
	defaultSlotNum  = 60
	defaultChanSize = 100
)

var (
	// ErrInvalidParam denotes that param is invalid.
	ErrInvalidParam = errors.New("asynctimer: param is invalid")
	// ErrShortDelay denotes that timer's delay is too short.
	ErrShortDelay = errors.New("asynctimer: delay time is too short")
)

var defaultTimeWheel *TimeWheel

func init() {
	var err error
	defaultTimeWheel, err = NewTimeWheel(defaultInterval, defaultSlotNum)
	if err != nil {
		panic(err)
	}
	defaultTimeWheel.Start()
}

// Add adds timer to the default time wheel. Timer becomes effective after
// having been added. Note that time precision is 1 second, if the required
// precision is less than 1s, a custom time wheel should be used.
func Add(timer *Timer) error {
	return defaultTimeWheel.Add(timer)
}

// Del deletes timer from the default time wheel. After having been deleted,
// timer becomes invalid, and the resource will be released.
func Del(timer *Timer) {
	defaultTimeWheel.Del(timer)
}

// Stop stops the default time wheel.
func Stop() {
	defaultTimeWheel.Stop()
}

// Callback is asynchronous callback function when timer expired.
type Callback func(data interface{})

// Timer is an async timer.
type Timer struct {
	data          interface{}
	expiredHandle Callback
	begin         atomic.Time
	circle        int
	delay         time.Duration
	timeout       time.Duration
	isActive      bool
}

// NewTimer creates an async timer with data and expiredHandle function. After timeout
// time, the timer will be expired, and expiredHandle will be called with argument data.
// Note that the timer becomes effective after having been added to time wheel.
func NewTimer(data interface{}, expiredHandle Callback, timeout time.Duration) *Timer {
	return &Timer{
		data:          data,
		expiredHandle: expiredHandle,
		timeout:       timeout,
		delay:         timeout,
	}
}

// TimeWheel manages all async timers.
type TimeWheel struct {
	now         time.Time
	timersToAdd chan *Timer
	timersToDel chan *Timer
	quit        chan struct{}
	ticker      *time.Ticker
	timerToSlot map[*Timer]*slot
	slots       []*slot
	interval    time.Duration
	slotNum     int
	currSlot    int
}

// NewTimeWheel creates a time wheel which manages async timers. Time wheel's round
// time is equal to slotNum*interval. Interval means duration of each tick of time wheel,
// slotNum means numbers of slots in a round.
func NewTimeWheel(interval time.Duration, slotNum int) (*TimeWheel, error) {
	if interval <= 0 || slotNum <= 0 {
		return nil, fmt.Errorf("%w, NewTimeWheel error: interval and slotNum should greater than 0",
			ErrInvalidParam)
	}

	t := &TimeWheel{
		now:         time.Now(),
		interval:    interval,
		slotNum:     slotNum,
		currSlot:    0,
		timersToAdd: make(chan *Timer, defaultChanSize),
		timersToDel: make(chan *Timer, defaultChanSize),
		quit:        make(chan struct{}),
	}
	t.timerToSlot = make(map[*Timer]*slot)
	t.slots = make([]*slot, t.slotNum)
	for i := 0; i < t.slotNum; i++ {
		t.slots[i] = newSlot()
	}
	return t, nil
}

// Start starts the time wheel. Make sure to start the time wheel before using it.
func (t *TimeWheel) Start() {
	t.ticker = time.NewTicker(t.interval)
	go t.run()
}

// Add adds timer to time wheel. Timer becomes effective after having been added
// to time wheel.
func (t *TimeWheel) Add(timer *Timer) error {
	if timer.isActive {
		timer.begin.Store(t.now)
		// Reset timer delay to timer timeout.
		timer.delay = timer.timeout
		return nil
	}
	if timer.delay < t.interval {
		return ErrShortDelay
	}
	timer.isActive = true
	// Reset timer delay to timer timeout.
	timer.delay = timer.timeout
	timer.delay = timer.delay.Round(t.interval)
	timer.begin.Store(t.now)
	t.timersToAdd <- timer
	return nil
}

// Del deletes timer from time wheel. After having been deleted from the wheel,
// timer become invalid, and the resource will be released.
func (t *TimeWheel) Del(timer *Timer) {
	timer.isActive = false
	t.timersToDel <- timer
}

// Stop stops the time wheel, all related resource managed by the time wheel will be released.
func (t *TimeWheel) Stop() {
	close(t.quit)
}

func (t *TimeWheel) del(timer *Timer) {
	if s, ok := t.timerToSlot[timer]; ok {
		s.del(timer)
		delete(t.timerToSlot, timer)
	}
}

func (t *TimeWheel) run() {
	for {
		select {
		case <-t.quit:
			t.quitHandle()
			return
		case <-t.ticker.C:
			t.tickHandle()
		case timer := <-t.timersToAdd:
			t.addTimerHandle(timer)
		case timer := <-t.timersToDel:
			t.delTimerHandle(timer)
		}
	}
}

func (t *TimeWheel) quitHandle() {
	t.ticker.Stop()
	t.timerToSlot = make(map[*Timer]*slot)
	for _, s := range t.slots {
		s.timers = make(map[*Timer]struct{})
	}
}

func (t *TimeWheel) tickHandle() {
	t.now = t.now.Add(t.interval)
	t.currSlot = (t.currSlot + 1) % t.slotNum
	s := t.slots[t.currSlot]
	for timer := range s.timers {
		if timer.circle != 0 {
			timer.circle--
			continue
		}
		s.del(timer)
		delete(t.timerToSlot, timer)
		timer.isActive = false
		go t.checkExpire(timer)
	}
}

func (t *TimeWheel) checkExpire(timer *Timer) {
	if timer.expiredHandle == nil {
		return
	}
	actual := t.now.Sub(timer.begin.Load())
	// Expired.
	if actual >= timer.timeout {
		timer.expiredHandle(timer.data)
		return
	}
	// Not expired, add timer to time wheel again.
	timer.delay = timer.timeout - actual
	if err := t.Add(timer); err != nil {
		timer.expiredHandle(timer.data)
	}
}

func (t *TimeWheel) addTimerHandle(timer *Timer) {
	t.del(timer)
	i, c := t.calIndexAndCircle(timer)
	timer.circle = c
	t.slots[i].add(timer)
	t.timerToSlot[timer] = t.slots[i]
}

func (t *TimeWheel) calIndexAndCircle(timer *Timer) (int, int) {
	delay := int(timer.delay.Milliseconds())
	interval := int(t.interval.Milliseconds())
	index := (t.currSlot + delay/interval) % t.slotNum
	circle := (delay - interval) / interval / t.slotNum
	return index, circle
}

func (t *TimeWheel) delTimerHandle(timer *Timer) {
	t.del(timer)
}

type slot struct {
	timers map[*Timer]struct{}
}

func newSlot() *slot {
	return &slot{timers: make(map[*Timer]struct{})}
}

func (s *slot) add(t *Timer) {
	s.timers[t] = struct{}{}
}

func (s *slot) del(t *Timer) {
	delete(s.timers, t)
}
