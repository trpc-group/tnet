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

// Copyright (c) 2019 Andy Pan
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build linux || freebsd || dragonfly || darwin
// +build linux freebsd dragonfly darwin

package netutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// SockaddrSize is the size of socket address, it use size of IPv6 socket
	// address for compatibility because IPv6 socket address is longer than IPv4.
	SockaddrSize = unix.SizeofSockaddrInet6
)

// SockaddrToTCPOrUnixAddr converts a Sockaddr to a net.TCPAddr or net.UnixAddr.
// Returns nil if conversion fails.
func SockaddrToTCPOrUnixAddr(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		ip := sockaddrInet4ToIP(sa)
		return &net.TCPAddr{IP: ip, Port: sa.Port}
	case *unix.SockaddrInet6:
		ip, zone := sockaddrInet6ToIPAndZone(sa)
		return &net.TCPAddr{IP: ip, Port: sa.Port, Zone: zone}
	case *unix.SockaddrUnix:
		return &net.UnixAddr{Name: sa.Name, Net: "unix"}
	}
	return nil
}

// SockaddrToUDPAddr converts a Sockaddr to a net.UDPAddr.
// Returns nil if conversion fails.
func SockaddrToUDPAddr(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		ip := sockaddrInet4ToIP(sa)
		return &net.UDPAddr{IP: ip, Port: sa.Port}
	case *unix.SockaddrInet6:
		ip, zone := sockaddrInet6ToIPAndZone(sa)
		return &net.UDPAddr{IP: ip, Port: sa.Port, Zone: zone}
	}
	return nil
}

// sockaddrInet4ToIPAndZone converts a SockaddrInet4 to a net.IP.
// It returns nil if conversion fails.
func sockaddrInet4ToIP(sa *unix.SockaddrInet4) net.IP {
	ip := make([]byte, 16)
	// V4InV6Prefix
	ip[10] = 0xff
	ip[11] = 0xff
	copy(ip[12:16], sa.Addr[:])
	return ip
}

// sockaddrInet6ToIPAndZone converts a SockaddrInet6 to a net.IP with IPv6 Zone.
// It returns nil if conversion fails.
func sockaddrInet6ToIPAndZone(sa *unix.SockaddrInet6) (net.IP, string) {
	ip := make([]byte, 16)
	copy(ip, sa.Addr[:])
	return ip, IP6ZoneToString(int(sa.ZoneId))
}

// IP6ZoneToString converts an IP6 Zone unix int to a net string.
// returns "" if zone is 0.
func IP6ZoneToString(zone int) string {
	if zone == 0 {
		return ""
	}
	if ifi, err := net.InterfaceByIndex(zone); err == nil {
		return ifi.Name
	}
	return int2decimal(uint(zone))
}

// StringToZoneID converts a IPv6 zone string to Zone unix int.
// returns 0 if zone is ""
func StringToZoneID(zone string) (uint32, error) {
	if zone == "" {
		return 0, nil
	}
	if ifi, err := net.InterfaceByName(zone); err == nil {
		return uint32(ifi.Index), nil
	}
	n, err := strconv.Atoi(zone)
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}

// Convert int to decimal string.
func int2decimal(i uint) string {
	if i == 0 {
		return "0"
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; i > 0; i /= 10 {
		bp--
		b[bp] = byte(i%10) + '0'
	}
	return string(b[bp:])
}

// LittleToBigEndian converts Little-Endian to Big-Endian.
func LittleToBigEndian(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)
	return binary.BigEndian.Uint16(b)
}

// BigToLittleEndian converts Big-Endian to Little-Endian.
func BigToLittleEndian(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return binary.LittleEndian.Uint16(b)
}

// SockaddrSliceToUDPAddr converts sockaddr byte slice to net.Addr.
func SockaddrSliceToUDPAddr(sockaddr []byte) (net.Addr, error) {
	if len(sockaddr) != SockaddrSize {
		return nil, errors.New("invalid sockaddr")
	}
	addr := &net.UDPAddr{}
	familyData := sockaddr[:2]
	family := (*uint16)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&familyData)).Data))
	switch *family {
	case unix.AF_INET:
		sockaddrInet4 := (*unix.RawSockaddrInet4)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&sockaddr)).Data))
		addr.IP = sockaddrInet4.Addr[:]
		addr.Port = int(BigToLittleEndian(sockaddrInet4.Port))
	case unix.AF_INET6:
		sockaddrInet6 := (*unix.RawSockaddrInet6)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&sockaddr)).Data))
		addr.IP = sockaddrInet6.Addr[:]
		addr.Port = int(BigToLittleEndian(sockaddrInet6.Port))
		addr.Zone = IP6ZoneToString(int(sockaddrInet6.Scope_id))
	default:
		err := fmt.Errorf("unknown net family")
		return nil, err
	}
	return addr, nil
}

// UDPAddrToSockaddrSlice converts net.UDPAddr struct to sockaddr byte slice.
func UDPAddrToSockaddrSlice(addr *net.UDPAddr) ([]byte, error) {
	sa := make([]byte, SockaddrSize)
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		if err := buildSockaddrSliceIPv4(sa, ipv4, addr.Port); err != nil {
			return nil, err
		}
	} else {
		zoneID, err := StringToZoneID(addr.Zone)
		if err != nil {
			return nil, err
		}
		if err := buildSockaddrSliceIPv6(sa, addr.IP.To16(), addr.Port, zoneID); err != nil {
			return nil, err
		}
	}
	return sa, nil
}

// UnixSockaddrToSockaddrSlice converts unix.Sockaddr to sockaddr byte slice
func UnixSockaddrToSockaddrSlice(unixSockaddr unix.Sockaddr, sockaddr []byte) error {
	switch us := unixSockaddr.(type) {
	case *unix.SockaddrInet4:
		return buildSockaddrSliceIPv4(sockaddr, us.Addr[:], us.Port)
	case *unix.SockaddrInet6:
		return buildSockaddrSliceIPv6(sockaddr, us.Addr[:], us.Port, us.ZoneId)
	default:
		return errors.New("addr type is not support")
	}
}

// buildSockaddrSliceIPv4 build IPv4 sockaddr byte slice with ip, port.
func buildSockaddrSliceIPv4(sockaddr []byte, ip []byte, port int) error {
	if len(sockaddr) < SockaddrSize {
		return errors.New("sockaddr length not enough")
	}
	binary.LittleEndian.PutUint16(sockaddr[:2], unix.AF_INET)
	binary.BigEndian.PutUint16(sockaddr[2:4], uint16(port))
	copy(sockaddr[4:8], ip)
	return nil
}

// buildSockaddrSliceIPv6 build IPv6 sockaddr byte slice with ip, port, zoneID.
func buildSockaddrSliceIPv6(sockaddr []byte, ip []byte, port int, zoneID uint32) error {
	if len(sockaddr) < SockaddrSize {
		return errors.New("sockaddr length not enough")
	}
	binary.LittleEndian.PutUint16(sockaddr[:2], unix.AF_INET6)
	binary.BigEndian.PutUint16(sockaddr[2:4], uint16(port))
	copy(sockaddr[8:24], ip)
	binary.BigEndian.PutUint32(sockaddr[24:], zoneID)
	return nil
}

// TestableNetwork checks whether the network is testable, only used for unit test.
func TestableNetwork(network string) bool {
	switch network {
	case "unix":
		return true
	case "tcp4", "udp4":
		return hasIPv4Addr()
	case "tcp6", "udp6":
		return hasIPv6Addr()
	case "tcp", "udp":
		return hasIPv6Addr() || hasIPv4Addr()
	default:
		return false
	}
}

func hasIPv4Addr() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ip, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ip.IP.To4() != nil {
			return true
		}
	}
	return false
}

func hasIPv6Addr() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ip, ok := addr.(*net.IPNet)
		if !ok || ip.IP.To4() != nil {
			continue
		}
		return true
	}
	return false
}

// AddrToSockAddr convert net addr to sockAddr
func AddrToSockAddr(laddr net.Addr, raddr net.Addr) (unix.Sockaddr, error) {
	switch raddr := raddr.(type) {
	case *net.TCPAddr:
		return tcpAddrToSockAddr(laddr, raddr)
	case *net.UDPAddr:
		return udpAddrToSockAddr(laddr, raddr)
	default:
		return nil, errors.New("addr type is not support")
	}
}

func tcpAddrToSockAddr(laddr net.Addr, raddr net.Addr) (unix.Sockaddr, error) {
	lTCPAddr, lok := laddr.(*net.TCPAddr)
	rTCPAddr, rok := raddr.(*net.TCPAddr)
	if !lok || !rok {
		return nil, fmt.Errorf("laddr and raddr are not both tcp addr, laddr is %T, raddr is %T", laddr, raddr)
	}
	family, err := getAndCompareFamily(lTCPAddr.IP, rTCPAddr.IP)
	if err != nil {
		return nil, fmt.Errorf("get and compare family of laddr and raddr: %w", err)
	}
	return ipToSockaddr(family, rTCPAddr.IP, rTCPAddr.Port, rTCPAddr.Zone)
}

func udpAddrToSockAddr(laddr net.Addr, raddr net.Addr) (unix.Sockaddr, error) {
	lUDPAddr, lOK := laddr.(*net.UDPAddr)
	rUDPAddr, rOK := raddr.(*net.UDPAddr)
	if !lOK || !rOK {
		return nil, fmt.Errorf("laddr and raddr are not both udp addr, laddr is %T, raddr is %T", laddr, raddr)
	}
	family, err := getAndCompareFamily(lUDPAddr.IP, rUDPAddr.IP)
	if err != nil {
		return nil, fmt.Errorf("get and compare family of laddr and raddr: %w", err)
	}
	return ipToSockaddr(family, rUDPAddr.IP, rUDPAddr.Port, rUDPAddr.Zone)
}

func getAndCompareFamily(lIP net.IP, rIP net.IP) (int, error) {
	lFamily, rFamily := getFamily(lIP), getFamily(rIP)
	if lFamily != rFamily {
		return -1, fmt.Errorf("IP family mismatch: laddr family is %v(%v), raddr family is %v(%v)",
			family2String(lFamily), lFamily, family2String(rFamily), rFamily)
	}
	return rFamily, nil
}

func family2String(family int) string {
	switch family {
	case unix.AF_INET:
		return "AF_INET"
	case unix.AF_INET6:
		return "AF_INET6"
	default:
		return "UNKOWN"
	}
}

// Copy from golang source code: tcpsock_posix.go
func getFamily(ip net.IP) int {
	if len(ip) <= net.IPv4len {
		return unix.AF_INET
	}
	if ip.To4() != nil {
		return unix.AF_INET
	}
	return unix.AF_INET6
}

// Copy from golang source code: tcpsock_posix.go
func ipToSockaddr(family int, ip net.IP, port int, zone string) (unix.Sockaddr, error) {
	switch family {
	case unix.AF_INET:
		if len(ip) == 0 {
			ip = net.IPv4zero
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return nil, fmt.Errorf("non-IPv4 address:%s", ip.String())
		}
		sa := &unix.SockaddrInet4{Port: port}
		copy(sa.Addr[:], ip4)
		return sa, nil
	case unix.AF_INET6:
		// In general, an IP wildcard address, which is either
		// "0.0.0.0" or "::", means the entire IP addressing
		// space. For some historical reason, it is used to
		// specify "any available address" on some operations
		// of IP node.
		//
		// When the IP node supports IPv4-mapped IPv6 address,
		// we allow a listener to listen to the wildcard
		// address of both IP addressing spaces by specifying
		// IPv6 wildcard address.
		if len(ip) == 0 || ip.Equal(net.IPv4zero) {
			ip = net.IPv6zero
		}
		// We accept any IPv6 address including IPv4-mapped
		// IPv6 address.
		ip6 := ip.To16()
		if ip6 == nil {
			return nil, fmt.Errorf("non-IPv6 address:%s", ip.String())
		}
		zoneID, err := StringToZoneID(zone)
		if err != nil {
			return nil, err
		}
		sa := &unix.SockaddrInet6{Port: port, ZoneId: zoneID}
		copy(sa.Addr[:], ip6)
		return sa, nil
	}
	return nil, fmt.Errorf("invalid address family:%s", ip.String())
}

// ValidateTCP validates that listener is listening on TCP.
func ValidateTCP(listener net.Listener) error {
	switch network := listener.Addr().Network(); network {
	case "tcp", "tcp4", "tcp6":
		return nil
	default:
		return fmt.Errorf("expected listen on TCP, actual listen on %s", network)
	}
}

// ValidateUDP validates that conn is listening on UDP.
func ValidateUDP(conn net.PacketConn) error {
	switch network := conn.LocalAddr().Network(); network {
	case "udp", "udp4", "udp6":
		return nil
	default:
		return fmt.Errorf("expected listen on UDP, actual listen on %s", network)
	}
}
