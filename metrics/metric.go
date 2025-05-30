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

// Package metrics provides a lot of tnet runtime monitoring data,
// such as monitoring the efficiency of batch reads and writes,
// which is a good tool for performance tuning.
package metrics

import (
	"time"

	"go.uber.org/atomic"
	"trpc.group/trpc-go/tnet/log"
)

// All metrics definitions.
const (
	// The following constants are TCP metrics.

	TCPReadvCalls = iota
	TCPReadvFails
	TCPReadvBytes
	TCPWritevCalls
	TCPWritevFails
	TCPWritevBlocks
	TCPWriteNotify
	TCPOnWriteCalls
	TCPFlushCalls
	TCPConnsCreate
	TCPConnsClose
	TCPPostponeWriteOff
	TCPPostponeWriteOn

	// The following constants are  UDP metrics.

	UDPRecvMMsgCalls
	UDPRecvMMsgFails
	UDPRecvMMsgPackets
	UDPRecvMsgCalls
	UDPRecvMsgFails
	UDPRecvMsgPackets
	UDPWriteToCalls
	UDPWriteToFails
	UDPSendMMsgCalls
	UDPSendMMsgFails
	UDPSendMMsgPackets

	// The following constants are  Epoll metrics.

	EpollWait
	EpollNoWait
	EpollEvents
	TaskAssigned

	// Keep it last.

	Max
)

var (
	metrics [Max]atomic.Uint64
)

// Add metrics counter.
func Add(name int, delta uint64) {
	if name >= Max {
		return
	}
	metrics[name].Add(delta)
}

// Get one metric counter.
func Get(name int) uint64 {
	if name >= Max {
		return 0
	}
	return metrics[name].Load()
}

// GetAll get all metrics.
func GetAll() [Max]uint64 {
	var m [Max]uint64
	for i := range metrics {
		m[i] = metrics[i].Load()
	}
	return m
}

// ShowMetricsOfPeriod shows metric info of duration d from now on.
// It will block d duration, and then prints metrics info.
func ShowMetricsOfPeriod(d time.Duration) {
	old := GetAll()
	<-time.After(d)
	new := GetAll()
	var m [Max]uint64
	for i := range metrics {
		m[i] = new[i] - old[i]
	}
	showAll(m)
}

// ShowMetrics shows metric info in console.
func ShowMetrics() {
	m := GetAll()
	showAll(m)
}

func showAll(m [Max]uint64) {
	log.Debug("######### tnet metrics (", time.Now().Format("2006-01-02 15:04:05"), ") ###########")
	showTCPMetrics(m)
	showUDPMetrics(m)
	showEpollMetrics(m)
	log.Debugf("%-59s: %d", "# number of task assigned (doTask)", m[TaskAssigned])
}

func showTCPMetrics(m [Max]uint64) {
	log.Debugf("%-59s: %d", "# TCP - number of Readv system calls", m[TCPReadvCalls])
	log.Debugf("%-59s: %d", "# TCP - number of failed Readv system calls", m[TCPReadvFails])
	readvSucc := m[TCPReadvCalls] - m[TCPReadvFails]
	if readvSucc > 0 {
		log.Debugf("%-59s: %dB", "# TCP - Readv efficiency", m[TCPReadvBytes]/readvSucc)
	}
	log.Debugf("%-59s: %d", "# TCP - number of Writev system calls", m[TCPWritevCalls])
	log.Debugf("%-59s: %d", "# TCP - number of blocks sent by Writev", m[TCPWritevBlocks])
	log.Debugf("%-59s: %d", "# TCP - number of failed Writev system calls", m[TCPWritevFails])
	writevSucc := m[TCPWritevCalls] - m[TCPWritevFails]
	if writevSucc > 0 {
		log.Debugf("%-59s: %.2f", "# TCP - Writev efficiency", float64(m[TCPWritevBlocks])/float64(writevSucc))
	}
	log.Debugf("%-59s: %d", "# TCP - number of epoll_ctl on write event", m[TCPWriteNotify])
	log.Debugf("%-59s: %d", "# TCP - number of tcpOnWrite calls", m[TCPOnWriteCalls])
	log.Debugf("%-59s: %d", "# TCP - number of connections created", m[TCPConnsCreate])
	log.Debugf("%-59s: %d", "# TCP - number of connections closed", m[TCPConnsClose])
	log.Debugf("%-59s: %d", "# TCP - number of times postpone write switched off", m[TCPPostponeWriteOff])
	log.Debugf("%-59s: %d", "# TCP - number of times postpone write switched on", m[TCPPostponeWriteOn])
}

func showUDPMetrics(m [Max]uint64) {
	// MMsg log.
	log.Debugf("%-59s: %d", "# UDP - number of RecvMMsg system calls", m[UDPRecvMMsgCalls])
	log.Debugf("%-59s: %d", "# UDP - number of failed RecvMMsg system calls", m[UDPRecvMMsgFails])
	recvMMsgSucc := m[UDPRecvMMsgCalls] - m[UDPRecvMMsgFails]
	if recvMMsgSucc > 0 {
		log.Debugf("%-59s: %.2f", "# UDP - RecvMMsg efficiency", float64(m[UDPRecvMMsgPackets])/float64(recvMMsgSucc))
	}
	log.Debugf("%-59s: %d", "# UDP - number of SendMMsg system calls", m[UDPSendMMsgCalls])
	log.Debugf("%-59s: %d", "# UDP - number of failed SendMMsg system calls", m[UDPSendMMsgFails])
	sendMMsgSucc := m[UDPSendMMsgCalls] - m[UDPSendMMsgFails]
	if sendMMsgSucc > 0 {
		log.Debugf("%-59s: %.2f", "# UDP - SendMMsg efficiency", float64(m[UDPSendMMsgPackets])/float64(sendMMsgSucc))
	}
	// Msg log.
	log.Debugf("%-59s: %d", "# UDP - number of RecvMsg system calls", m[UDPRecvMsgCalls])
	log.Debugf("%-59s: %d", "# UDP - number of failed RecvMsg system calls", m[UDPRecvMsgFails])
	recvMsgSucc := m[UDPRecvMsgCalls] - m[UDPRecvMsgFails]
	if recvMsgSucc > 0 {
		log.Debugf("%-59s: %.2f", "# UDP - RecvMsg efficiency", float64(m[UDPRecvMsgPackets])/float64(recvMsgSucc))
	}
	// UDP write log.
	log.Debugf("%-59s: %d", "# UDP - number of WriteTo system calls", m[UDPWriteToCalls])
	log.Debugf("%-59s: %d", "# UDP - number of failed WriteTo system calls", m[UDPWriteToFails])
}

func showEpollMetrics(m [Max]uint64) {
	log.Debugf("%-59s: %d", "# EPOLL - number of epoll_wait returns (tag:b)", m[EpollWait])
	log.Debugf("%-59s: %d", "# EPOLL - number of epoll_wait called with msc=0 (tag:a)", m[EpollNoWait])
	log.Debugf("%-59s: %d", "# EPOLL - number of total events", m[EpollEvents])
	if (m[EpollWait]) > 0 {
		log.Debugf("%-59s: %.2f%%", "# EPOLL - a/b * 100%", float32(m[EpollNoWait])*100/float32(m[EpollWait]))
		log.Debugf("%-59s: %.2f", "# EPOLL - average events number per epoll_wait",
			float32(m[EpollEvents])/float32(m[EpollWait]))
	}
}
