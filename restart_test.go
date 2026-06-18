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

package tnet

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type serveOnlyService struct{}

func (serveOnlyService) Serve(context.Context) error { return nil }

var _ Service = serveOnlyService{}

func TestCleanAndAppendEnv(t *testing.T) {
	got := cleanAndAppendEnv([]string{
		"A=1",
		gracefulRestartFDEnv + "=old",
		"B=2",
	}, gracefulRestartFDEnv, gracefulRestartFD)
	require.Equal(t, []string{"A=1", "B=2", gracefulRestartFDEnv + "=" + gracefulRestartFD}, got)
}

func TestWithGracefulRestartTimeout(t *testing.T) {
	var opts options
	opts.setDefault()
	require.Equal(t, defaultGracefulRestartTimeout, opts.gracefulRestartTimeout)

	WithGracefulRestartTimeout(time.Millisecond).f(&opts)
	require.Equal(t, time.Millisecond, opts.gracefulRestartTimeout)
}

func TestServicesImplementRestartable(t *testing.T) {
	ln, err := Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tcpSvc, err := NewTCPService(ln, nil)
	require.NoError(t, err)
	require.Implements(t, (*Restartable)(nil), tcpSvc)
	require.NoError(t, tcpSvc.(*tcpservice).close())

	lns, err := ListenPackets("udp", "127.0.0.1:0", false)
	require.NoError(t, err)
	udpSvc, err := NewUDPService(lns, nil)
	require.NoError(t, err)
	require.Implements(t, (*Restartable)(nil), udpSvc)
	require.NoError(t, udpSvc.(*udpservice).close())
}

func TestTCPServiceRestartStartError(t *testing.T) {
	ln, err := Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	svc, err := NewTCPService(ln, nil)
	require.NoError(t, err)
	ts := svc.(*tcpservice)

	origCommand := execCommand
	t.Cleanup(func() { execCommand = origCommand })
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("tnet-test-binary-does-not-exist")
	}

	err = svc.(Restartable).Restart(context.Background())
	require.Error(t, err)
	require.False(t, ts.restarting.Load())
}

func TestTCPServiceRestartKeepsActiveConnection(t *testing.T) {
	ln, err := Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	svc, err := NewTCPService(ln, echoTCP, WithGracefulRestartTimeout(10*time.Millisecond))
	require.NoError(t, err)
	defer svc.(*tcpservice).close()

	lastCmd := mockRestartCommand(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := serveTCP(t, svc, ctx)
	client, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	require.NoError(t, err)

	assertTCPEcho(t, client, "before_restart")

	restartDone := make(chan error, 1)
	go func() {
		restartDone <- svc.(Restartable).Restart(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-restartDone:
		t.Fatalf("restart returned before active connection drained: %v", err)
	default:
	}

	assertTCPEcho(t, client, "after_restart")
	require.NoError(t, client.Close())

	select {
	case err := <-restartDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("restart did not finish after active connection closed")
	}
	requireRestartCommand(t, lastCmd())

	select {
	case err := <-serveDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("service did not return after restart drain")
	}
}

func TestTCPServiceWaitConnectionsContext(t *testing.T) {
	ln, err := Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	svc, err := NewTCPService(ln, nil)
	require.NoError(t, err)
	ts := svc.(*tcpservice)
	ts.conns[1001] = &tcpconn{nfd: netFD{fd: 1001}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ts.waitConnections(ctx)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("waitConnections did not return after context cancellation")
	}

	ts.mu.Lock()
	delete(ts.conns, 1001)
	ts.mu.Unlock()
}

func TestUDPServiceRestartClosesConns(t *testing.T) {
	lns := make([]PacketConn, 0, 2)
	for i := 0; i < 2; i++ {
		rawConn, err := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, err)
		conn, err := NewPacketConn(rawConn)
		require.NoError(t, err)
		lns = append(lns, conn)
	}
	t.Cleanup(func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	})

	svc, err := newUDPService(lns, nil, WithGracefulRestartTimeout(0))
	require.NoError(t, err)
	lastCmd := mockRestartCommand(t)

	err = svc.(Restartable).Restart(context.Background())
	require.NoError(t, err)
	requireRestartCommand(t, lastCmd())
	for _, ln := range lns {
		require.False(t, ln.IsActive())
	}
}

func echoTCP(conn Conn) error {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	_, err = conn.Write(buf[:n])
	return err
}

func assertTCPEcho(t *testing.T, conn net.Conn, message string) {
	t.Helper()
	_, err := conn.Write([]byte(message))
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	buf := make([]byte, len(message))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, message, string(buf))
}

func serveTCP(t *testing.T, svc Service, ctx context.Context) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- svc.Serve(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	return done
}

func mockRestartCommand(t *testing.T) func() *exec.Cmd {
	t.Helper()
	origCommand := execCommand
	var last *exec.Cmd
	t.Cleanup(func() {
		execCommand = origCommand
		if last != nil && last.Process != nil && last.ProcessState == nil {
			_ = last.Wait()
		}
	})
	execCommand = func(string, ...string) *exec.Cmd {
		last = exec.Command(os.Args[0], "-test.run=TestRestartHelperProcess", "--", "restart-helper")
		return last
	}
	return func() *exec.Cmd {
		return last
	}
}

func requireRestartCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	require.NotNil(t, cmd)
	require.Contains(t, cmd.Env, gracefulRestartFDEnv+"="+gracefulRestartFD)
	require.Len(t, cmd.ExtraFiles, 1)
}

func TestRestartHelperProcess(t *testing.T) {
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "restart-helper" {
		os.Exit(0)
	}
}
