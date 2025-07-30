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
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// IsBigEndian is a flag determining if value is in Big Endian
var IsBigEndian bool

func init() {
	var i uint16 = 0x0100
	ptr := unsafe.Pointer(&i) //#nosec:G103
	if *(*byte)(ptr) == 0x01 {
		// we are on the big endian machine
		IsBigEndian = true
	}
}

func reverse(s []byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func Test_byteToTime(t *testing.T) {
	timeb := []byte{63, 155, 21, 96, 0, 0, 0, 0, 52, 156, 191, 42, 0, 0, 0, 0}
	if IsBigEndian {
		// reverse two int64 individually
		reverse(timeb[0:8])
		reverse(timeb[8:16])
	}
	res, err := byteToTime(timeb)
	if err != nil {
		t.Fatalf("byteToTime() returned an unexpected error: %v", err)
	}

	expectedNano := int64(1612028735717200436)
	if res.UnixNano() != expectedNano {
		t.Fatalf("expected UnixNano %d, got %d", expectedNano, res.UnixNano())
	}
}

func Test_ReadTXtimestamp(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	err = EnableSWTimestamps(connFd)
	if err != nil {
		t.Fatalf("EnableSWTimestamps failed: %v", err)
	}

	start := time.Now()
	txts, attempts, err := ReadTXtimestamp(connFd)
	duration := time.Since(start)

	if !txts.IsZero() {
		t.Fatalf("expected zero time, got %v", txts)
	}
	if attempts != defaultTXTS {
		t.Fatalf("expected %d attempts, got %d", defaultTXTS, attempts)
	}
	errStr := fmt.Sprintf("no TX timestamp found after %d tries", defaultTXTS)
	if err == nil || !strings.Contains(err.Error(), errStr) {
		t.Fatalf("expected error to contain %q, but got: %v", errStr, err)
	}
	minDuration := time.Duration(AttemptsTXTS) * TimeoutTXTS
	if duration < minDuration {
		t.Fatalf("expected duration to be >= %v, but got %v", minDuration, duration)
	}

	AttemptsTXTS = 10
	TimeoutTXTS = 5 * time.Millisecond

	start = time.Now()
	txts, attempts, err = ReadTXtimestamp(connFd)
	duration = time.Since(start)
	if !txts.IsZero() {
		t.Fatalf("expected zero time, got %v", txts)
	}
	if attempts != 10 {
		t.Fatalf("expected 10 attempts, got %d", attempts)
	}
	errStr = fmt.Sprintf("no TX timestamp found after %d tries", 10)
	if err == nil || !strings.Contains(err.Error(), errStr) {
		t.Fatalf("expected error to contain %q, but got: %v", errStr, err)
	}
	minDuration = time.Duration(AttemptsTXTS) * TimeoutTXTS
	if duration < minDuration {
		t.Fatalf("expected duration to be >= %v, but got %v", minDuration, duration)
	}

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	_, err = conn.WriteTo([]byte{}, addr)
	if err != nil {
		t.Fatalf("conn.WriteTo failed: %v", err)
	}
	txts, attempts, err = ReadTXtimestamp(connFd)

	if txts.IsZero() {
		t.Fatal("expected a non-zero timestamp, but got a zero time")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if err != nil {
		t.Fatalf("ReadTXtimestamp() returned an unexpected error: %v", err)
	}
}

func Test_scmDataToTime(t *testing.T) {
	hwData := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		63, 155, 21, 96, 0, 0, 0, 0, 52, 156, 191, 42, 0, 0, 0, 0,
	}
	swData := []byte{
		63, 155, 21, 96, 0, 0, 0, 0, 52, 156, 191, 42, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	noData := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}

	if IsBigEndian {
		reverse(hwData[32:40])
		reverse(hwData[40:48])
		reverse(swData[0:8])
		reverse(swData[8:16])
	}

	tests := []struct {
		name    string
		data    []byte
		want    int64
		wantErr bool
	}{
		{"hardware timestamp", hwData, 1612028735717200436, false},
		{"software timestamp", swData, 1612028735717200436, false},
		{"zero timestamp", noData, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := scmDataToTime(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("scmDataToTime() returned an unexpected error: %v", err)
				}
				if res.UnixNano() != tt.want {
					t.Fatalf("expected UnixNano %d, got %d", tt.want, res.UnixNano())
				}
			}
		})
	}
}

func TestEnableSWTimestampsRx(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	err = EnableSWTimestampsRx(connFd)
	if err != nil {
		t.Fatalf("EnableSWTimestampsRx failed: %v", err)
	}

	timestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING)
	newTimestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING_NEW)

	if timestampsEnabled+newTimestampsEnabled <= 0 {
		t.Fatal("None of the socket options is set")
	}
}

func TestEnableSWTimestamps(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	err = EnableSWTimestamps(connFd)
	if err != nil {
		t.Fatalf("EnableSWTimestamps failed: %v", err)
	}

	timestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING)
	newTimestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING_NEW)

	if timestampsEnabled+newTimestampsEnabled <= 0 {
		t.Fatal("None of the socket options is set")
	}
}

func TestEnableTimestamps(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	// SOFTWARE
	err = EnableTimestamps(SW, connFd, "lo")
	if err != nil {
		t.Fatalf("EnableTimestamps(SW) failed: %v", err)
	}

	timestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING)
	newTimestampsEnabled, _ := unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING_NEW)

	if timestampsEnabled+newTimestampsEnabled <= 0 {
		t.Fatal("None of the socket options is set for SW")
	}

	// SOFTWARE_RX
	err = EnableTimestamps(SWRX, connFd, "lo")
	if err != nil {
		t.Fatalf("EnableTimestamps(SWRX) failed: %v", err)
	}

	timestampsEnabled, _ = unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING)
	newTimestampsEnabled, _ = unix.GetsockoptInt(connFd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING_NEW)

	if timestampsEnabled+newTimestampsEnabled <= 0 {
		t.Fatal("None of the socket options is set for SWRX")
	}

	// Unsupported
	err = EnableTimestamps(42, connFd, "lo")
	expectedErr := fmt.Errorf("unrecognized timestamp type: Unsupported")
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestSocketControlMessageTimestamp(t *testing.T) {
	if timestamping != unix.SO_TIMESTAMPING_NEW {
		t.Skip("This test supports SO_TIMESTAMPING_NEW only. No sample of SO_TIMESTAMPING")
	}

	var b []byte
	var toob int

	switch runtime.GOARCH {
	case "amd64":
		b = []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x41, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x79, 0xab, 0x24, 0x68, 0x0, 0x0, 0x0, 0x0, 0xfc, 0xab, 0xf9, 0x8, 0x0, 0x0, 0x0, 0x0, 0x3c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x29, 0x0, 0x0, 0x0, 0x19, 0x0, 0x0, 0x0, 0x2a, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		toob = len(b)
	default:
		t.Skip("This test checks amd64 platform only")
	}

	ts, err := socketControlMessageTimestamp(b, toob)
	if err != nil {
		t.Fatalf("socketControlMessageTimestamp failed: %v", err)
	}
	expectedNano := int64(1747233657150580220)
	if ts.UnixNano() != expectedNano {
		t.Fatalf("expected UnixNano %d, got %d", expectedNano, ts.UnixNano())
	}
}

func TestSocketControlMessageTimestampFail(t *testing.T) {
	if timestamping != unix.SO_TIMESTAMPING_NEW {
		t.Skip("This test supports SO_TIMESTAMPING_NEW only. No sample of SO_TIMESTAMPING")
	}

	_, err := socketControlMessageTimestamp(make([]byte, 16), 16)
	if !errors.Is(err, errNoTimestamp) {
		t.Fatalf("expected error to be %v, but got %v", errNoTimestamp, err)
	}
}

func TestReadPacketWithRXTimestamp(t *testing.T) {
	request := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 42}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("localhost"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	err = EnableSWTimestampsRx(connFd)
	if err != nil {
		t.Fatalf("EnableSWTimestampsRx failed: %v", err)
	}

	err = unix.SetNonblock(connFd, false)
	if err != nil {
		t.Fatalf("unix.SetNonblock failed: %v", err)
	}

	timeout := 1 * time.Second
	cconn, err := net.DialTimeout("udp", conn.LocalAddr().String(), timeout)
	if err != nil {
		t.Fatalf("net.DialTimeout failed: %v", err)
	}
	defer checkClose(t, cconn)
	_, err = cconn.Write(request)
	if err != nil {
		t.Fatalf("cconn.Write failed: %v", err)
	}

	data, returnaddr, nowKernelTimestamp, err := ReadPacketWithRXTimestamp(connFd)
	if err != nil {
		t.Fatalf("ReadPacketWithRXTimestamp failed: %v", err)
	}

	if !bytes.Equal(request, data) {
		t.Fatalf("We should have the same request arriving on the server. Expected %v, got %v", request, data)
	}
	if time.Now().Unix()/10 != nowKernelTimestamp.Unix()/10 {
		t.Fatalf("kernel timestamps should be within 10s. Now: %d, Kernel: %d", time.Now().Unix(), nowKernelTimestamp.Unix())
	}
	requireEqualNetAddrSockAddr(t, cconn.LocalAddr(), returnaddr)
}

func TestReadPacketWithRXTXTimestamp(t *testing.T) {
	request := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 42}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("localhost"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	err = EnableSWTimestamps(connFd)
	if err != nil {
		t.Fatalf("EnableSWTimestamps failed: %v", err)
	}

	err = unix.SetNonblock(connFd, false)
	if err != nil {
		t.Fatalf("unix.SetNonblock failed: %v", err)
	}

	timeout := 1 * time.Second
	cconn, err := net.DialTimeout("udp", conn.LocalAddr().String(), timeout)
	if err != nil {
		t.Fatalf("net.DialTimeout failed: %v", err)
	}
	defer checkClose(t, cconn)
	_, err = cconn.Write(request)
	if err != nil {
		t.Fatalf("cconn.Write failed: %v", err)
	}

	data, returnaddr, nowKernelTimestamp, err := ReadPacketWithRXTimestamp(connFd)
	if err != nil {
		t.Fatalf("ReadPacketWithRXTimestamp failed: %v", err)
	}
	if !bytes.Equal(request, data) {
		t.Fatalf("We should have the same request arriving on the server. Expected %v, got %v", request, data)
	}
	if time.Now().Unix()/10 != nowKernelTimestamp.Unix()/10 {
		t.Fatalf("kernel timestamps should be within 10s. Now: %d, Kernel: %d", time.Now().Unix(), nowKernelTimestamp.Unix())
	}
	requireEqualNetAddrSockAddr(t, cconn.LocalAddr(), returnaddr)

	_, err = conn.WriteTo(request, cconn.LocalAddr())
	if err != nil {
		t.Fatalf("conn.WriteTo failed: %v", err)
	}

	txts, attempts, err := ReadTXtimestamp(connFd)
	if txts.IsZero() {
		t.Fatal("expected a non-zero timestamp, but got a zero time")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if err != nil {
		t.Fatalf("ReadTXtimestamp() returned an unexpected error: %v", err)
	}
}

func TestReadHWTimestampCaps(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("localhost"), Port: 0})
	if err != nil {
		t.Fatalf("net.ListenUDP failed: %v", err)
	}
	defer checkClose(t, conn)

	connFd, err := ConnFd(conn)
	if err != nil {
		t.Fatalf("ConnFd failed: %v", err)
	}

	rxFilters, txType, err := ioctlHWTimestampCaps(connFd, "lo")
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
	if txType != 0 {
		t.Fatalf("expected txType to be 0, got %d", txType)
	}
	if rxFilters != 0 {
		t.Fatalf("expected rxFilters to be 0, got %d", rxFilters)
	}
}

func TestScmDataToSeqID(t *testing.T) {
	hwData := []byte{0x2a, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xd2, 0x4, 0x0, 0x0}
	seqID, err := scmDataToSeqID(hwData)
	if err != nil {
		t.Fatalf("scmDataToSeqID failed: %v", err)
	}
	if seqID != 1234 {
		t.Fatalf("expected seqID 1234, got %d", seqID)
	}
}

func TestScmDataToSeqIDErrornoNotENOMSG(t *testing.T) {
	hwData := []byte{0x26, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xd2, 0x4, 0x0, 0x0}
	_, err := scmDataToSeqID(hwData)
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
	expectedStr := "expected ENOMSG but got function not implemented"
	if !strings.Contains(err.Error(), expectedStr) {
		t.Fatalf("expected error to contain %q, but got: %v", expectedStr, err)
	}
}

func TestSeqIDSocketControlMessage(t *testing.T) {
	soob := make([]byte, unix.CmsgSpace(SizeofSeqID))
	seqID := uint32(8765)
	var sockControlMsg []byte
	SeqIDSocketControlMessage(seqID, soob)

	switch runtime.GOARCH {
	case "amd64":
		sockControlMsg = []byte{0x14, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x51, 0x0, 0x0, 0x0, 0x3d, 0x22, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	case "386":
		sockControlMsg = []byte{0x10, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x51, 0x0, 0x0, 0x0, 0x3d, 0x22, 0x0, 0x0}
	default:
		t.Skip("This test supports 386/amd64 platform only")
	}

	if !bytes.Equal(sockControlMsg, soob) {
		t.Fatalf("expected soob to be %v, got %v", sockControlMsg, soob)
	}
}

func TestSocketControlMessageSeqIDTimestamp(t *testing.T) {
	tboob := 128
	seqID := uint32(3248)
	switch runtime.GOARCH {
	case "amd64":
		sockControlMsgs := []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x41, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x69, 0x75, 0x23, 0x68, 0x0, 0x0, 0x0, 0x0, 0x7b, 0xcb, 0x4, 0x6, 0x0, 0x0, 0x0, 0x0, 0x3c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x29, 0x0, 0x0, 0x0, 0x19, 0x0, 0x0, 0x0, 0x2a, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xb0, 0xc, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		ts, err := socketControlMessageSeqIDTimestamp(sockControlMsgs, tboob, seqID)
		if err != nil {
			t.Fatalf("socketControlMessageSeqIDTimestamp failed: %v", err)
		}
		expectedNano := int64(1747154281100977531)
		if ts.UnixNano() != expectedNano {
			t.Fatalf("expected UnixNano %d, got %d", expectedNano, ts.UnixNano())
		}
	default:
		t.Skip("This test supports amd64 platform only")
	}
}
