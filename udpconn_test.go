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

package tnet_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/tnet"
)

type udpTestCase struct {
	servHandle    func(t *testing.T, conn tnet.PacketConn, ch chan int) error
	clientHandle  func(t *testing.T, conn net.Conn, ch chan int)
	ctrlHandle    func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int)
	name          string
	isTnetCliConn bool
}

func doUDPTestCase(t *testing.T, tt udpTestCase, serverOpts ...tnet.Option) {
	var (
		serverConn  []tnet.PacketConn
		clientConn  net.Conn
		waitChannel = make(chan int)
	)
	// Set up server.
	serverAddr := getTestAddr()
	lns, err := tnet.ListenPackets("udp", serverAddr, true)
	require.Nil(t, err)
	serverConn = lns

	s, err := tnet.NewUDPService(lns, func(conn tnet.PacketConn) error {
		return tt.servHandle(t, conn, waitChannel)
	}, serverOpts...)
	require.Nil(t, err)

	serverAddr = lns[0].LocalAddr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx)

	// Set up client.
	time.Sleep(time.Millisecond * 5)
	if tt.isTnetCliConn {
		clientConn, err = tnet.DialUDP("udp", serverAddr, 5*time.Second)
	} else {
		clientConn, err = net.DialTimeout("udp", serverAddr, 5*time.Second)
	}
	require.Nil(t, err)
	defer clientConn.Close()
	// Run the client handler.
	if tt.clientHandle != nil {
		tt.clientHandle(t, clientConn, waitChannel)
	}
	// Run the control handler.
	if tt.ctrlHandle != nil {
		tt.ctrlHandle(t, serverConn, clientConn, waitChannel)
	}
}

func TestUDPConnClose_IOClose(t *testing.T) {
	doUDPTestCase(t, udpTestCase{
		name: "close by request routine",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			conn.Close()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for _, s := range server {
				if !s.IsActive() {
					return
				}
			}
			assert.FailNow(t, "at least one udpconn should be closed")
		},
	})
}

func TestUDPConnClose_APIClose(t *testing.T) {
	doUDPTestCase(t, udpTestCase{
		name: "close by businiss routine",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			ch <- 1
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for _, s := range server {
				assert.Nil(t, s.Close())
				// Close twice.
				assert.Nil(t, s.Close())
				assert.False(t, s.IsActive())
			}
		},
	})
}

func TestUDPConnClose_ReadPacketBlocked(t *testing.T) {
	doUDPTestCase(t, udpTestCase{
		name: "close by server with ReadPacket() blocked",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			ch <- 1
			_, _, err := conn.ReadPacket()
			assert.NotNil(t, err)
			return nil
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			// make sure ReadPacket is blocked
			time.Sleep(time.Millisecond * 100)
			for _, c := range server {
				assert.Nil(t, c.Close())
				assert.False(t, c.IsActive())
			}
		},
	})
}

func TestUDPConnClose_ConcurrentClose(t *testing.T) {
	doUDPTestCase(t, udpTestCase{
		name: "close by write error",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			wg := sync.WaitGroup{}
			for i := 0; i <= 10; i++ {
				wg.Add(1)
				go func() {
					assert.Nil(t, conn.Close())
					wg.Done()
				}()
			}
			wg.Wait()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
			<-ch
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond * 100)
			for _, s := range server {
				if !s.IsActive() {
					return
				}
			}
			assert.FailNow(t, "at least one udpconn should be closed")
		},
	})
}

func TestUDPConnWrite_ServHandleErr(t *testing.T) {
	doUDPTestCase(t, udpTestCase{
		name: "server close connection when handle error",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			return errors.New("handle error")
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			_, err := conn.Write(helloWorld)
			assert.Nil(t, err)
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			time.Sleep(time.Millisecond * 100)
			for _, s := range server {
				if !s.IsActive() {
					return
				}
			}
			assert.FailNow(t, "at least one udpconn should be closed")
		},
		isTnetCliConn: true,
	})
}

var pkgnum = 100

func TestUDPConnRead_ReadPacket(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "call ReadPacket",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			p, addr, err := conn.ReadPacket()
			require.Nil(t, err)
			defer p.Free()
			req, err := p.Data()
			require.Nil(t, err)
			n, err := conn.WriteTo(req, addr)
			require.Equal(t, len(req), n)
			require.Nil(t, err)
			atomic.AddUint32(&count, 1)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	})
}

func TestUDPConnRead_ReadFrom(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "call ReadFrom",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			req := make([]byte, len(helloWorld))
			_, addr, err := conn.ReadFrom(req)
			require.Nil(t, err)

			rsp := make([]byte, len(req))
			copy(rsp, req)
			n, err := conn.WriteTo(rsp, addr)
			require.Equal(t, len(rsp), n)
			require.Nil(t, err)

			atomic.AddUint32(&count, 1)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	})
}

func TestUDPConnRead_ConcurrentReadFrom(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "concurrent call peek and skip",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			readFunc := func() {
				for {
					req := make([]byte, len(helloWorld))
					_, addr, err := conn.ReadFrom(req)
					if err != nil {
						return
					}
					_, err = conn.WriteTo(req, addr)
					assert.Nil(t, err)
					atomic.AddUint32(&count, 1)
				}
			}
			for i := 0; i < 10; i++ {
				go readFunc()
			}
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	})
}

func TestUDPConnWrite_AsyncWrite(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "concurrent Write",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			req := make([]byte, len(helloWorld))
			_, addr, err := conn.ReadFrom(req)
			assert.Nil(t, err)
			require.Equal(t, helloWorld, req)
			go func() {
				_, err := conn.WriteTo(req, addr)
				assert.Nil(t, err)
				atomic.AddUint32(&count, 1)
			}()
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	})
}

func TestUDPConnWrite_AsyncWrite_Flush(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "concurrent Write",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			req := make([]byte, len(helloWorld))
			_, addr, err := conn.ReadFrom(req)
			assert.Nil(t, err)
			require.Equal(t, helloWorld, req)
			go func() {
				_, err := conn.WriteTo(req, addr)
				assert.Nil(t, err)
				atomic.AddUint32(&count, 1)
			}()
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	},
		tnet.WithFlushWrite(true))
}

func Test_udpconn_ReadWriteAfterClose(t *testing.T) {
	conns, err := tnet.ListenPackets("udp", getTestAddr(), false)
	assert.Nil(t, err)

	conn := conns[0]
	conn.Close()
	_, _, err = conn.ReadPacket()
	assert.NotNil(t, err)

	buf := make([]byte, 1)
	_, _, err = conn.ReadFrom(buf)
	assert.NotNil(t, err)

	_, err = conn.WriteTo(buf, nil)
	assert.NotNil(t, err)
}

func Test_udpconn_ReadWriteNil(t *testing.T) {
	conns, err := tnet.ListenPackets("udp", getTestAddr(), false)
	assert.Nil(t, err)
	conn := conns[0]
	defer conn.Close()

	n, addr, err := conn.ReadFrom(nil)
	assert.Zero(t, n)
	assert.Nil(t, addr)
	assert.Nil(t, err)

	n, err = conn.WriteTo(nil, nil)
	assert.Zero(t, n)
	assert.Nil(t, err)

	_, err = conn.Write(nil)
	assert.NotNil(t, err)
}

func TestUDPConnReadTimeout(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			err = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doUDPTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestTNetPacketConnTimeout(t *testing.T) {
	conns, err := tnet.ListenPackets("udp", getTestAddr(), false)
	assert.Nil(t, err)
	conn := conns[0]

	conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

	data := make([]byte, 1)
	_, _, err = conn.ReadFrom(data)
	assert.NotNil(t, err)
}

func TestNetPacketConnTimeout(t *testing.T) {
	udpAddr, _ := net.ResolveUDPAddr("udp", getTestAddr())
	conn, err := net.ListenUDP("udp", udpAddr)
	assert.Nil(t, err)

	conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

	data := make([]byte, 1)
	_, _, err = conn.ReadFrom(data)
	assert.NotNil(t, err)
}

func TestUDPConnReadTimeout_Zero(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Time{})
			assert.Nil(t, err)
		},
	}
	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConnReadTimeout_notActive(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Close()
			err := conn.SetReadDeadline(time.Time{})
			assert.Equal(t, tnet.ErrConnClosed, err)
		},
	}
	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConnReadTimeout_Reset(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			data := make([]byte, 10)
			conn.Read(data)
			conn.Read(data)

			err = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			_, err = conn.Read(data)

			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doUDPTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConnReadTimeout_clean(t *testing.T) {
	var count int
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			if count != 0 {
				time.Sleep(time.Hour)
			}
			req := make([]byte, len(helloWorld))
			n, addr, err := conn.ReadFrom(req)
			assert.Nil(t, err)
			n, err = conn.WriteTo(req[:n], addr)
			assert.Equal(t, len(helloWorld), n)
			assert.Nil(t, err)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)

			_, err = conn.Write(helloWorld)
			assert.Nil(t, err)

			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.Nil(t, err)

			err = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doUDPTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConn_SetDeadline(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)

			time.Sleep(50 * time.Millisecond)

			_, err = conn.Write(helloWorld)
			assert.NotNil(t, err)

			data := make([]byte, 10)
			_, err = conn.Read(data)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doUDPTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConn_SetWriteDeadline(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			err := conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
			assert.Nil(t, err)

			time.Sleep(50 * time.Millisecond)

			_, err = conn.Write(helloWorld)
			assert.NotNil(t, err)
		},
	}

	tt.name = "net"
	tt.isTnetCliConn = false
	doUDPTestCase(t, tt)

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConn_SetDeadline_err(t *testing.T) {
	tt := udpTestCase{
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			time.Sleep(time.Hour)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Close()
			err := conn.SetDeadline(time.Now().Add(50 * time.Millisecond))
			assert.NotNil(t, err)
			err = conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
			assert.NotNil(t, err)
		},
	}

	tt.name = "tnet"
	tt.isTnetCliConn = true
	doUDPTestCase(t, tt)
}

func TestUDPConnMetaData(t *testing.T) {
	lns, err := tnet.ListenPackets("udp", getTestAddr(), false)
	require.Nil(t, err)
	ln := lns[0]

	c, err := tnet.DialUDP(ln.LocalAddr().Network(), ln.LocalAddr().String(), time.Second)
	assert.Nil(t, err)

	tc, ok := c.(tnet.PacketConn)
	assert.True(t, ok)

	tc.SetMetaData(helloWorld)
	ctx := tc.GetMetaData()

	b, ok := ctx.([]byte)
	assert.True(t, ok)
	assert.Equal(t, helloWorld, b)
}

func TestUDPConnRead_NonBlocking(t *testing.T) {
	var count uint32
	doUDPTestCase(t, udpTestCase{
		name: "call ReadPacket",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			p, addr, err := conn.ReadPacket()
			if err != nil {
				return err
			}
			defer p.Free()
			req, err := p.Data()
			assert.Nil(t, err)
			n, err := conn.WriteTo(req, addr)
			require.Equal(t, len(req), n)
			require.Nil(t, err)
			atomic.AddUint32(&count, 1)
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			for i := 0; i < pkgnum; i++ {
				_, err := conn.Write(helloWorld)
				require.Nil(t, err)
			}
		},
		ctrlHandle: func(t *testing.T, server []tnet.PacketConn, client net.Conn, ch chan int) {
			for {
				if int(atomic.LoadUint32(&count)) == pkgnum {
					break
				}
			}
		},
	}, tnet.WithNonBlocking(true))
}

func TestUDPConn_OnClose(t *testing.T) {
	onClosed := func(conn tnet.PacketConn) error {
		_, _, err := conn.ReadPacket()
		assert.Equal(t, tnet.ErrConnClosed, err)
		addr, _ := net.ResolveUDPAddr("udp", getTestAddr())
		_, err = conn.WriteTo(helloWorld, addr)
		assert.Equal(t, tnet.ErrConnClosed, err)
		data := conn.GetMetaData()
		if data != nil {
			assert.Equal(t, helloWorld, data.([]byte))
		}
		return nil
	}
	doUDPTestCase(t, udpTestCase{
		name: "call nonblocking read",
		servHandle: func(t *testing.T, conn tnet.PacketConn, ch chan int) error {
			conn.SetMetaData(helloWorld)
			conn.Close()
			ch <- 1
			return nil
		},
		clientHandle: func(t *testing.T, conn net.Conn, ch chan int) {
			conn.Write(helloWorld)
			<-ch
		},
	},
		tnet.WithOnUDPClosed(onClosed),
	)
}
