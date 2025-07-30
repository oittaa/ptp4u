/*
Copyright (c) Facebook, Inc. and its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package timestamp

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

func requireEqualNetAddrSockAddr(t *testing.T, n net.Addr, s unix.Sockaddr) {
	t.Helper() // Mark as a test helper function.
	uaddr, ok := n.(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected net.Addr to be *net.UDPAddr, but it was %T", n)
	}

	saddr6, ok := s.(*unix.SockaddrInet6)
	if ok {
		if !bytes.Equal(uaddr.IP.To16(), saddr6.Addr[:]) {
			t.Fatalf("IPv6 mismatch: expected %v, got %v", uaddr.IP.To16(), saddr6.Addr[:])
		}
		if uaddr.Port != saddr6.Port {
			t.Fatalf("Port mismatch: expected %d, got %d", uaddr.Port, saddr6.Port)
		}
		return
	}

	saddr4, ok := s.(*unix.SockaddrInet4)
	if !ok {
		t.Fatalf("Sockaddr was not an IPv4 or IPv6 address type")
	}
	if !bytes.Equal(uaddr.IP.To4(), saddr4.Addr[:]) {
		t.Fatalf("IPv4 mismatch: expected %v, got %v", uaddr.IP.To4(), saddr4.Addr[:])
	}
	if uaddr.Port != saddr4.Port {
		t.Fatalf("Port mismatch: expected %d, got %d", uaddr.Port, saddr4.Port)
	}
}

func TestConnFd(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("localhost"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer conn.Close()

	connfd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd() failed: %v", err)
	}
	if !(connfd > 0) {
		t.Fatalf("connection fd must be > 0, but got %d", connfd)
	}
}

func TestIPToSockaddr(t *testing.T) {
	ip4 := net.ParseIP("127.0.0.1")
	ip6 := net.ParseIP("::1")
	port := 123

	expectedSA4 := &unix.SockaddrInet4{Port: port}
	copy(expectedSA4.Addr[:], ip4.To4())

	expectedSA6 := &unix.SockaddrInet6{Port: port}
	copy(expectedSA6.Addr[:], ip6.To16())

	sa4 := IPToSockaddr(ip4, port)
	sa6 := IPToSockaddr(ip6, port)

	if !reflect.DeepEqual(expectedSA4, sa4) {
		t.Fatalf("expected sockaddr %v, got %v", expectedSA4, sa4)
	}
	if !reflect.DeepEqual(expectedSA6, sa6) {
		t.Fatalf("expected sockaddr %v, got %v", expectedSA6, sa6)
	}
}

func TestAddrToSockaddr(t *testing.T) {
	ip4 := netip.MustParseAddr("192.168.0.1")
	ip6 := netip.MustParseAddr("::1")
	port := 123

	expectedSA4 := &unix.SockaddrInet4{Port: port}
	copy(expectedSA4.Addr[:], ip4.AsSlice())

	expectedSA6 := &unix.SockaddrInet6{Port: port}
	copy(expectedSA6.Addr[:], ip6.AsSlice())

	sa4 := AddrToSockaddr(ip4, port)
	sa6 := AddrToSockaddr(ip6, port)

	if !reflect.DeepEqual(expectedSA4, sa4) {
		t.Fatalf("expected sockaddr %v, got %v", expectedSA4, sa4)
	}
	if !reflect.DeepEqual(expectedSA6, sa6) {
		t.Fatalf("expected sockaddr %v, got %v", expectedSA6, sa6)
	}
}

func TestSockaddrToIP(t *testing.T) {
	ip4 := net.ParseIP("127.0.0.1")
	ip6 := net.ParseIP("::1")
	port := 123

	sa4 := IPToSockaddr(ip4, port)
	sa6 := IPToSockaddr(ip6, port)

	if ip4.String() != SockaddrToIP(sa4).String() {
		t.Fatalf("expected IP %s, got %s", ip4, SockaddrToIP(sa4))
	}
	if ip6.String() != SockaddrToIP(sa6).String() {
		t.Fatalf("expected IP %s, got %s", ip6, SockaddrToIP(sa6))
	}
}

func TestSockaddrToPort(t *testing.T) {
	ip4 := net.ParseIP("127.0.0.1")
	ip6 := net.ParseIP("::1")
	port := 123

	sa4 := IPToSockaddr(ip4, port)
	sa6 := IPToSockaddr(ip6, port)

	if port != SockaddrToPort(sa4) {
		t.Fatalf("expected port %d, got %d", port, SockaddrToPort(sa4))
	}
	if port != SockaddrToPort(sa6) {
		t.Fatalf("expected port %d, got %d", port, SockaddrToPort(sa6))
	}
}

func TestTimestampUnmarshalText(t *testing.T) {
	var ts Timestamp
	if ts.Type() != "timestamp" {
		t.Fatalf("expected type 'timestamp', got %s", ts.Type())
	}

	err := ts.UnmarshalText([]byte("hardware"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if HW != ts {
		t.Fatalf("expected timestamp %v, got %v", HW, ts)
	}
	if HW.String() != ts.String() {
		t.Fatalf("expected string %s, got %s", HW, ts)
	}

	err = ts.UnmarshalText([]byte("hardware_rx"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if HWRX != ts {
		t.Fatalf("expected timestamp %v, got %v", HWRX, ts)
	}
	if HWRX.String() != ts.String() {
		t.Fatalf("expected string %s, got %s", HWRX, ts)
	}

	err = ts.UnmarshalText([]byte("software"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if SW != ts {
		t.Fatalf("expected timestamp %v, got %v", SW, ts)
	}
	if SW.String() != ts.String() {
		t.Fatalf("expected string %s, got %s", SW, ts)
	}

	err = ts.UnmarshalText([]byte("software_rx"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if SWRX != ts {
		t.Fatalf("expected timestamp %v, got %v", SWRX, ts)
	}
	if SWRX.String() != ts.String() {
		t.Fatalf("expected string %s, got %s", SWRX, ts)
	}

	err = ts.UnmarshalText([]byte("nope"))
	expectedErr := errors.New("unknown timestamp type \"nope\"")
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %q, got %q", expectedErr, err)
	}
	// Check we didn't change the value
	if SWRX != ts {
		t.Fatalf("expected timestamp to remain %v, but got %v", SWRX, ts)
	}
}

func TestTimestampMarshalText(t *testing.T) {
	text, err := HW.MarshalText()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(text) != "hardware" {
		t.Fatalf("expected 'hardware', got %s", string(text))
	}

	text, err = HWRX.MarshalText()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(text) != "hardware_rx" {
		t.Fatalf("expected 'hardware_rx', got %s", string(text))
	}

	text, err = SW.MarshalText()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(text) != "software" {
		t.Fatalf("expected 'software', got %s", string(text))
	}

	text, err = SWRX.MarshalText()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(text) != "software_rx" {
		t.Fatalf("expected 'software_rx', got %s", string(text))
	}

	if Timestamp(42).String() != "Unsupported" {
		t.Fatalf("expected 'Unsupported', got %s", Timestamp(42).String())
	}
	text, err = Timestamp(42).MarshalText()
	expectedErr := errors.New("unknown timestamp type \"Unsupported\"")
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %q, got %q", expectedErr, err)
	}
	if string(text) != "Unsupported" {
		t.Fatalf("expected 'Unsupported', got %s", string(text))
	}
}

func TestNewSockaddrWithPort(t *testing.T) {
	oldSA := &unix.SockaddrInet4{Addr: [4]byte{1, 2, 3, 4}, Port: 4567}
	newSA := NewSockaddrWithPort(oldSA, 8901)
	newSA4, ok := newSA.(*unix.SockaddrInet4)
	if !ok {
		t.Fatalf("expected newSA to be *unix.SockaddrInet4")
	}

	if oldSA.Addr != newSA4.Addr {
		t.Fatalf("expected Addr to be equal, got %v and %v", oldSA.Addr, newSA4.Addr)
	}
	if newSA4.Port != 8901 {
		t.Fatalf("expected Port to be 8901, got %d", newSA4.Port)
	}

	// changing the original should not change the new one
	oldSA.Addr[0] = 42
	if oldSA.Addr == newSA4.Addr {
		t.Fatalf("expected Addr to be different after modification, but they were equal")
	}
}
