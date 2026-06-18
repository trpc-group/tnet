//go:build linux
// +build linux

package tnet

import (
	"net"
	"testing"
	"time"

	"trpc.group/trpc-go/tnet/internal/poller"
)

func TestUDPConnWriteToDropsFailedPacketAndContinues(t *testing.T) {
	raw, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := newUDPConn(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.postpone.Set(true)
	if err := conn.SetOnRequest(func(PacketConn) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := conn.schedule(); err != nil {
		t.Fatal(err)
	}

	receiver, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	goodAddr := receiver.LocalAddr()
	badAddr := &net.UDPAddr{IP: net.ParseIP("::1"), Port: 25000}
	first := []byte("first")
	second := []byte("second")

	if n, err := conn.WriteTo(first, goodAddr); err != nil || n != len(first) {
		t.Fatalf("WriteTo(first) = %d, %v; want %d, nil", n, err, len(first))
	}
	if n, err := conn.WriteTo([]byte("bad"), badAddr); err != nil || n != len("bad") {
		t.Fatalf("WriteTo(bad) = %d, %v; want %d, nil", n, err, len("bad"))
	}
	if n, err := conn.WriteTo(second, goodAddr); err != nil || n != len(second) {
		t.Fatalf("WriteTo(second) = %d, %v; want %d, nil", n, err, len(second))
	}

	got := readUDPPayloads(t, receiver, 2)
	if string(got[0]) != string(first) || string(got[1]) != string(second) {
		t.Fatalf("received payloads = %q, want [%q %q]", got, first, second)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn.outBuffer.LenRead() == 0 && !conn.writing.IsLocked() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn.outBuffer.LenRead() != 0 {
		t.Fatalf("outBuffer length = %d, want 0", conn.outBuffer.LenRead())
	}
	if conn.writing.IsLocked() {
		t.Fatal("writing lock is still held")
	}
	if err := conn.nfd.Control(poller.ModReadable); err != nil {
		t.Fatalf("udp fd was detached from poller: %v", err)
	}
}

func readUDPPayloads(t *testing.T, receiver net.PacketConn, n int) [][]byte {
	t.Helper()

	got := make([][]byte, 0, n)
	buf := make([]byte, defaultUDPBufferSize)
	deadline := time.Now().Add(2 * time.Second)
	for len(got) < n {
		if err := receiver.SetReadDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		read, _, err := receiver.ReadFrom(buf)
		if err != nil {
			t.Fatalf("read payload %d/%d: %v", len(got)+1, n, err)
		}
		payload := make([]byte, read)
		copy(payload, buf[:read])
		got = append(got, payload)
	}
	return got
}
