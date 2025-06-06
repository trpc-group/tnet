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

package tnet

import (
	"fmt"
	"net"
	"sync"
	"time"

	"trpc.group/trpc-go/tnet/internal/iovec"
	"trpc.group/trpc-go/tnet/internal/netutil"
	"trpc.group/trpc-go/tnet/internal/stat"
	"trpc.group/trpc-go/tnet/metrics"
)

// DialTCP connects to the address on the named network within the timeout.
// Valid networks for DialTCP are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only).
func DialTCP(network, address string, timeout time.Duration) (Conn, error) {
	reportDialTCP()
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("DialTCP: unknown network %s", network)
	}
	return dialTCP(network, address, timeout)
}

// DialUDP connects to the address on the named network within the timeout.
// Valid networks for DialUDP are "udp", "udp4" (IPv4-only), "udp6" (IPv6-only).
func DialUDP(network, address string, timeout time.Duration) (PacketConn, error) {
	reportDialUDP()
	switch network {
	case "udp", "udp4", "udp6":
	default:
		return nil, fmt.Errorf("DialUDP: unknown network %s", network)
	}
	return dialUDP(network, address, timeout)
}

func dialTCP(network, address string, timeout time.Duration) (Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial network %s, address %s with timeout %+v error: %w", network, address, timeout, err)
	}
	fd, err := netutil.GetFD(c)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("dial tcp get fd error: %w", err)
	}
	conn := &tcpconn{
		nfd: netFD{
			fd:      fd,
			fdtype:  fdTCP,
			sock:    c,
			laddr:   c.LocalAddr(),
			raddr:   c.RemoteAddr(),
			network: network,
		},
		readTrigger: make(chan struct{}, 1),
		writevData:  iovec.NewIOData(),
	}
	conn.inBuffer.Initialize()
	conn.outBuffer.Initialize()
	conn.closedReadBuf.Initialize(nil)
	if err := conn.nfd.Schedule(tcpOnRead, tcpOnWrite, tcpOnHup, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dial tcp net fd schedule error: %w", err)
	}
	metrics.Add(metrics.TCPConnsCreate, 1)
	return conn, nil
}

func dialUDP(network, address string, timeout time.Duration) (PacketConn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial network %s, address %s with timeout %+v error: %w", network, address, timeout, err)
	}
	fd, err := netutil.GetFD(c)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("dial udp get fd error: %w", err)
	}
	conn := &udpconn{
		nfd: netFD{
			fd:            fd,
			fdtype:        fdUDP,
			sock:          c,
			laddr:         c.LocalAddr(),
			raddr:         c.RemoteAddr(),
			network:       network,
			udpBufferSize: defaultUDPBufferSize,
		},
		readTrigger: make(chan struct{}, 1),
	}
	conn.inBuffer.Initialize()
	conn.outBuffer.Initialize()
	if err := conn.schedule(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dial udp net fd schedule error: %w", err)
	}
	return conn, nil
}

var (
	dialTCPReportOnce sync.Once
	dialUDPReportOnce sync.Once
)

func reportDialTCP() {
	dialTCPReportOnce.Do(func() {
		stat.Report(stat.ClientAttr, stat.TCPAttr)
	})
}

func reportDialUDP() {
	dialUDPReportOnce.Do(func() {
		stat.Report(stat.ClientAttr, stat.UDPAttr)
	})
}
