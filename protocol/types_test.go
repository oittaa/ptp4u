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

package protocol

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestSdoIDAndMsgType(t *testing.T) {
	sdoIDAndMsgType := NewSdoIDAndMsgType(MessageSignaling, 123)
	if sdoIDAndMsgType.MsgType() != MessageSignaling {
		t.Errorf("expected msg type %v, got %v", MessageSignaling, sdoIDAndMsgType.MsgType())
	}
}

func TestProbeMsgType(t *testing.T) {
	tests := []struct {
		in      []byte
		want    MessageType
		wantErr bool
	}{
		{
			in:      []byte{},
			wantErr: true,
		},
		{
			in:   []byte{0x0},
			want: MessageSync,
		},
		{
			in:   []byte{0xC},
			want: MessageSignaling,
		},
		{
			in:   []byte{0xBC},
			want: MessageSignaling,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("ProbeMsgType in=%#x", tt.in), func(t *testing.T) {
			got, err := ProbeMsgType(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Error("expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMessageTypeString(t *testing.T) {
	testCases := []struct {
		input    MessageType
		expected string
	}{
		{MessageSync, "SYNC"},
		{MessageDelayReq, "DELAY_REQ"},
		{MessagePDelayReq, "PDELAY_REQ"},
		{MessagePDelayResp, "PDELAY_RES"},
		{MessageFollowUp, "FOLLOW_UP"},
		{MessageDelayResp, "DELAY_RESP"},
		{MessagePDelayRespFollowUp, "PDELAY_RESP_FOLLOW_UP"},
		{MessageAnnounce, "ANNOUNCE"},
		{MessageSignaling, "SIGNALING"},
		{MessageManagement, "MANAGEMENT"},
	}

	for _, tc := range testCases {
		if tc.input.String() != tc.expected {
			t.Errorf("For %v, expected %q, but got %q", tc.input, tc.expected, tc.input.String())
		}
	}
}

func TestTLVTypeString(t *testing.T) {
	testCases := []struct {
		input    TLVType
		expected string
	}{
		{TLVManagement, "MANAGEMENT"},
		{TLVManagementErrorStatus, "MANAGEMENT_ERROR_STATUS"},
		{TLVOrganizationExtension, "ORGANIZATION_EXTENSION"},
		{TLVRequestUnicastTransmission, "REQUEST_UNICAST_TRANSMISSION"},
		{TLVGrantUnicastTransmission, "GRANT_UNICAST_TRANSMISSION"},
		{TLVCancelUnicastTransmission, "CANCEL_UNICAST_TRANSMISSION"},
		{TLVAcknowledgeCancelUnicastTransmission, "ACKNOWLEDGE_CANCEL_UNICAST_TRANSMISSION"},
		{TLVPathTrace, "PATH_TRACE"},
		{TLVAlternateTimeOffsetIndicator, "ALTERNATE_TIME_OFFSET_INDICATOR"},
	}

	for _, tc := range testCases {
		if tc.input.String() != tc.expected {
			t.Errorf("For %v, expected %q, but got %q", tc.input, tc.expected, tc.input.String())
		}
	}
}

func TestTimeSourceString(t *testing.T) {
	testCases := []struct {
		input    TimeSource
		expected string
	}{
		{TimeSourceAtomicClock, "ATOMIC_CLOCK"},
		{TimeSourceGNSS, "GNSS"},
		{TimeSourceTerrestrialRadio, "TERRESTRIAL_RADIO"},
		{TimeSourcePTP, "PTP"},
		{TimeSourceNTP, "NTP"},
		{TimeSourceHandSet, "HAND_SET"},
		{TimeSourceOther, "OTHER"},
		{TimeSourceInternalOscillator, "INTERNAL_OSCILLATOR"},
	}

	for _, tc := range testCases {
		if tc.input.String() != tc.expected {
			t.Errorf("For %v, expected %q, but got %q", tc.input, tc.expected, tc.input.String())
		}
	}
}

func TestPortStateString(t *testing.T) {
	testCases := []struct {
		input    PortState
		expected string
	}{
		{PortStateInitializing, "INITIALIZING"},
		{PortStateFaulty, "FAULTY"},
		{PortStateDisabled, "DISABLED"},
		{PortStateListening, "LISTENING"},
		{PortStatePreMaster, "PRE_MASTER"},
		{PortStateMaster, "MASTER"},
		{PortStatePassive, "PASSIVE"},
		{PortStateUncalibrated, "UNCALIBRATED"},
		{PortStateSlave, "SLAVE"},
	}

	for _, tc := range testCases {
		if tc.input.String() != tc.expected {
			t.Errorf("For %v, expected %q, but got %q", tc.input, tc.expected, tc.input.String())
		}
	}
}

func TestPortIdentityString(t *testing.T) {
	pi := PortIdentity{}
	if pi.String() != "000000.0000.000000-0" {
		t.Errorf("expected '000000.0000.000000-0', got %q", pi.String())
	}
	pi = PortIdentity{
		ClockIdentity: 5212879185253000328,
		PortNumber:    1,
	}
	if pi.String() != "4857dd.fffe.086488-1" {
		t.Errorf("expected '4857dd.fffe.086488-1', got %q", pi.String())
	}
}

func TestPortIdentityCompare(t *testing.T) {
	pi1 := PortIdentity{PortNumber: 1, ClockIdentity: 5212879185253000328}
	pi2 := PortIdentity{PortNumber: 1, ClockIdentity: 0}
	pi3 := PortIdentity{PortNumber: 69, ClockIdentity: 0}
	if pi1.Compare(pi1) != 0 {
		t.Error("pi1.Compare(pi1) should be 0")
	}
	if pi1.Compare(pi2) != 1 {
		t.Error("pi1.Compare(pi2) should be 1")
	}
	if pi2.Compare(pi3) != -1 {
		t.Error("pi2.Compare(pi3) should be -1")
	}
}

func TestPortIdentityLess(t *testing.T) {
	pi1 := PortIdentity{PortNumber: 1, ClockIdentity: 5212879185253000328}
	pi2 := PortIdentity{PortNumber: 1, ClockIdentity: 0}
	pi3 := PortIdentity{PortNumber: 69, ClockIdentity: 0}
	if pi1.Less(pi1) {
		t.Error("pi1.Less(pi1) should be false")
	}
	if pi1.Less(pi2) {
		t.Error("pi1.Less(pi2) should be false")
	}
	if !pi2.Less(pi1) {
		t.Error("pi2.Less(pi1) should be true")
	}
	if !pi2.Less(pi3) {
		t.Error("pi2.Less(pi3) should be true")
	}
	if pi3.Less(pi2) {
		t.Error("pi3.Less(pi2) should be false")
	}
}

func TestTimeIntervalNanoseconds(t *testing.T) {
	tests := []struct {
		in      TimeInterval
		want    float64
		wantStr string
	}{
		{
			in:      13697024,
			want:    209,
			wantStr: "TimeInterval(209.000ns)",
		},
		{
			in:      0x0000000000028000,
			want:    2.5,
			wantStr: "TimeInterval(2.500ns)",
		},
		{
			in:      -9240576,
			want:    -141,
			wantStr: "TimeInterval(-141.000ns)",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("TimeInterval.Nanoseconds t=%d", tt.in), func(t *testing.T) {
			got := tt.in.Nanoseconds()
			if got != tt.want {
				t.Errorf("Nanoseconds() got %v, want %v", got, tt.want)
			}
			if tt.in.String() != tt.wantStr {
				t.Errorf("String() got %q, want %q", tt.in.String(), tt.wantStr)
			}
			gotTI := NewTimeInterval(got)
			if gotTI != tt.in {
				t.Errorf("NewTimeInterval() got %v, want %v", gotTI, tt.in)
			}
		})
	}
}

func TestPTPSeconds(t *testing.T) {
	tests := []struct {
		in      PTPSeconds
		want    time.Time
		wantStr string
	}{
		{
			in:      [6]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x02},
			want:    time.Unix(2, 0),
			wantStr: fmt.Sprintf("PTPSeconds(%s)", time.Unix(2, 0)),
		},
		{
			in:      [6]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			want:    time.Time{},
			wantStr: "PTPSeconds(empty)",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("PTPSeconds t=%v", tt.in), func(t *testing.T) {
			got := tt.in.Time()
			if !got.Equal(tt.want) {
				t.Errorf("Time() got %v, want %v", got, tt.want)
			}
			if tt.in.String() != tt.wantStr {
				t.Errorf("String() got %q, want %q", tt.in.String(), tt.wantStr)
			}
			gotTS := NewPTPSeconds(got)
			if gotTS != tt.in {
				t.Errorf("NewPTPSeconds() got %v, want %v", gotTS, tt.in)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	tests := []struct {
		in      Timestamp
		want    time.Time
		wantStr string
	}{
		{
			in: Timestamp{
				Seconds:     [6]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x02},
				Nanoseconds: 1,
			},
			want:    time.Unix(2, 1),
			wantStr: fmt.Sprintf("Timestamp(%s)", time.Unix(2, 1)),
		},
		{
			in: Timestamp{
				Seconds:     [6]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
				Nanoseconds: 0,
			},
			want:    time.Time{},
			wantStr: "Timestamp(empty)",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Timestamp t=%v", tt.in), func(t *testing.T) {
			got := tt.in.Time()
			if !got.Equal(tt.want) {
				t.Errorf("Time() got %v, want %v", got, tt.want)
			}
			if tt.in.String() != tt.wantStr {
				t.Errorf("String() got %q, want %q", tt.in.String(), tt.wantStr)
			}
			gotTS := NewTimestamp(got)
			if gotTS != tt.in {
				t.Errorf("NewTimestamp() got %v, want %v", gotTS, tt.in)
			}
		})
	}
}

func TestCorrectionFromDuration(t *testing.T) {
	tests := []struct {
		in         time.Duration
		want       Correction
		wantTooBig bool
		wantStr    string
	}{
		{
			in:      time.Millisecond,
			want:    Correction(65536000000),
			wantStr: "Correction(1000000.000ns)",
		},
		{
			in:         50 * time.Hour,
			want:       Correction(0x7fffffffffffffff),
			wantTooBig: true,
			wantStr:    "Correction(Too big)",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Correction of %v", tt.in), func(t *testing.T) {
			got := NewCorrection(float64(tt.in))
			if got != tt.want {
				t.Errorf("NewCorrection() got %v, want %v", got, tt.want)
			}
			if got.String() != tt.wantStr {
				t.Errorf("String() got %q, want %q", got.String(), tt.wantStr)
			}
			gotNS := got.Nanoseconds()
			if tt.wantTooBig {
				if !math.IsInf(gotNS, 1) {
					t.Errorf("expected Nanoseconds() to be infinite, but got %v", gotNS)
				}
			} else {
				if time.Duration(gotNS) != tt.in {
					t.Errorf("Nanoseconds() got %v, want %v", time.Duration(gotNS), tt.in)
				}
			}
		})
	}
}

func TestDurationFromCorrection(t *testing.T) {
	tests := []struct {
		in   Correction
		want time.Duration
	}{
		{
			in:   Correction(65536000000),
			want: time.Millisecond,
		},
		{
			in:   Correction(0x7fffffffffffffff),
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Duration of %v", tt.in), func(t *testing.T) {
			got := tt.in.Duration()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogInterval(t *testing.T) {
	tests := []struct {
		in   LogInterval
		want float64 // seconds
	}{
		{in: 0, want: 1},
		{in: 1, want: 2},
		{in: 5, want: 32},
		{in: -1, want: 0.5},
		{in: -7, want: 0.0078125},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("LogInterval t=%d", tt.in), func(t *testing.T) {
			gotDuration := tt.in.Duration()
			if gotDuration.Seconds() != tt.want {
				t.Errorf("Duration() got %v, want %v", gotDuration.Seconds(), tt.want)
			}
			gotLI, err := NewLogInterval(gotDuration)
			if err != nil {
				t.Fatalf("NewLogInterval() unexpected error: %v", err)
			}
			if gotLI != tt.in {
				t.Errorf("NewLogInterval() got %v, want %v", gotLI, tt.in)
			}
		})
	}
}

func TestClockIdentity(t *testing.T) {
	macStr := "0c:42:a1:6d:7c:a6"
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		t.Fatalf("net.ParseMAC failed: %v", err)
	}
	got, err := NewClockIdentity(mac)
	if err != nil {
		t.Fatalf("NewClockIdentity failed: %v", err)
	}
	want := ClockIdentity(0xc42a1fffe6d7ca6)
	if want != got {
		t.Errorf("got %v, want %v", got, want)
	}
	wantStr := "0c42a1.fffe.6d7ca6"
	if wantStr != got.String() {
		t.Errorf("got %q, want %q", got.String(), wantStr)
	}
	back := got.MAC()
	if !reflect.DeepEqual(mac, back) {
		t.Errorf("got %v, want %v", back, mac)
	}
}

func TestPTPText(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		want    string
		wantErr bool
	}{
		{name: "no data", in: []byte{}, want: "", wantErr: true},
		{name: "empty", in: []byte{0}, want: "", wantErr: false},
		{name: "some text", in: []byte{4, 65, 108, 101, 120}, want: "Alex", wantErr: false},
		{name: "padding", in: []byte{3, 120, 101, 108, 0}, want: "xel", wantErr: false},
		{name: "non-ascii", in: []byte{3, 120, 255, 200, 0}, want: "x\xff\xc8", wantErr: false},
		{name: "too short", in: []byte{20, 120, 255, 200, 0}, want: "", wantErr: true},
		{name: "single", in: []byte{1, 65, 0}, want: "A", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var text PTPText
			err := text.UnmarshalBinary(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Error("expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if string(text) != tt.want {
					t.Errorf("got %q, want %q", string(text), tt.want)
				}

				gotBytes, err := text.MarshalBinary()
				if err != nil {
					t.Fatalf("MarshalBinary failed: %v", err)
				}
				if !reflect.DeepEqual(tt.in, gotBytes) {
					t.Errorf("got %v, want %v", gotBytes, tt.in)
				}
			}
		})
	}
}

func TestPortAddress(t *testing.T) {
	tests := []struct {
		name      string
		in        []byte
		want      *PortAddress
		wantIP    net.IP
		wantErr   bool
		wantIPErr bool
	}{
		{name: "no data", in: []byte{}, want: nil, wantErr: true},
		{name: "empty", in: []byte{0}, want: nil, wantErr: true},
		{name: "unsupported protocol", in: []byte{0x00, 0x04, 0x00, 0x04, 192, 168, 0, 1}, want: nil, wantErr: false, wantIPErr: true},
		{name: "ipv4 too long", in: []byte{0x00, 0x01, 0x00, 0x05, 192, 168, 0, 1, 0}, want: nil, wantErr: false, wantIPErr: true},
		{name: "ipv4 too short", in: []byte{0x00, 0x01, 0x00, 0x04, 192, 168, 0}, want: nil, wantErr: true, wantIPErr: false},
		{name: "ipv4", in: []byte{0x00, 0x01, 0x00, 0x04, 192, 168, 0, 1}, want: &PortAddress{NetworkProtocol: TransportTypeUDPIPV4, AddressLength: 4, AddressField: []byte{192, 168, 0, 1}}, wantIP: net.ParseIP("192.168.0.1"), wantErr: false},
		{name: "ipv6 too short", in: []byte{0x00, 0x02, 0x00, 0x10, 0x24, 0x01, 0xdb, 0x00, 0xff, 0xfe, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, want: nil, wantErr: true, wantIPErr: false},
		{name: "ipv6 too long", in: []byte{0x00, 0x02, 0x00, 0x11, 0x24, 0x01, 0xdb, 0x00, 0xff, 0xfe, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, want: nil, wantErr: false, wantIPErr: true},
		{name: "ipv6", in: []byte{0x00, 0x02, 0x00, 0x10, 0x24, 0x01, 0xdb, 0x00, 0xff, 0xfe, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, want: &PortAddress{NetworkProtocol: TransportTypeUDPIPV6, AddressLength: 16, AddressField: []byte{0x24, 0x01, 0xdb, 0x00, 0xff, 0xfe, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}, wantIP: net.ParseIP("2401:db00:fffe:123::"), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr PortAddress
			err := addr.UnmarshalBinary(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			ip, err := addr.IP()
			if tt.wantIPErr {
				if err == nil {
					t.Fatal("expected an IP error, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected IP error: %v", err)
			}

			if !reflect.DeepEqual(*tt.want, addr) {
				t.Errorf("got %+v, want %+v", addr, *tt.want)
			}

			if !tt.wantIP.Equal(ip) {
				t.Errorf("expect parsed IP %v to be equal to %v", ip, tt.wantIP)
			}

			gotBytes, err := addr.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary failed: %v", err)
			}
			if !reflect.DeepEqual(tt.in, gotBytes) {
				t.Errorf("got %v, want %v", gotBytes, tt.in)
			}
		})
	}
}

func TestClockAccuracyFromOffset(t *testing.T) {
	testCases := []struct {
		input    time.Duration
		expected ClockAccuracy
	}{
		{-8 * time.Nanosecond, ClockAccuracyNanosecond25},
		{42 * time.Nanosecond, ClockAccuracyNanosecond100},
		{-242 * time.Nanosecond, ClockAccuracyNanosecond250},
		{567 * time.Nanosecond, ClockAccuracyMicrosecond1},
		{2 * time.Microsecond, ClockAccuracyMicrosecond2point5},
		{8 * time.Microsecond, ClockAccuracyMicrosecond10},
		{11 * time.Microsecond, ClockAccuracyMicrosecond25},
		{-42 * time.Microsecond, ClockAccuracyMicrosecond100},
		{123 * time.Microsecond, ClockAccuracyMicrosecond250},
		{678 * time.Microsecond, ClockAccuracyMillisecond1},
		{2499 * time.Microsecond, ClockAccuracyMillisecond2point5},
		{-8 * time.Millisecond, ClockAccuracyMillisecond10},
		{24 * time.Millisecond, ClockAccuracyMillisecond25},
		{69 * time.Millisecond, ClockAccuracyMillisecond100},
		{222 * time.Millisecond, ClockAccuracyMillisecond250},
		{-999 * time.Millisecond, ClockAccuracySecond1},
		{10 * time.Second, ClockAccuracySecond10},
		{9 * time.Minute, ClockAccuracySecondGreater10},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Offset %v", tc.input), func(t *testing.T) {
			if got := ClockAccuracyFromOffset(tc.input); got != tc.expected {
				t.Errorf("For offset %v, expected %v, but got %v", tc.input, tc.expected, got)
			}
		})
	}
}

func TestClockAccuracyToDuration(t *testing.T) {
	testCases := []struct {
		input    ClockAccuracy
		expected time.Duration
	}{
		{ClockAccuracyNanosecond25, time.Nanosecond * 25},
		{ClockAccuracyNanosecond100, time.Nanosecond * 100},
		{ClockAccuracyNanosecond250, time.Nanosecond * 250},
		{ClockAccuracyMicrosecond1, time.Microsecond},
		{ClockAccuracyMicrosecond2point5, time.Nanosecond * 2500},
		{ClockAccuracyMicrosecond10, time.Microsecond * 10},
		{ClockAccuracyMicrosecond25, time.Microsecond * 25},
		{ClockAccuracyMicrosecond100, time.Microsecond * 100},
		{ClockAccuracyMicrosecond250, time.Microsecond * 250},
		{ClockAccuracyMillisecond1, time.Millisecond},
		{ClockAccuracyMillisecond2point5, time.Microsecond * 2500},
		{ClockAccuracyMillisecond10, time.Millisecond * 10},
		{ClockAccuracyMillisecond25, time.Millisecond * 25},
		{ClockAccuracyMillisecond100, time.Millisecond * 100},
		{ClockAccuracyMillisecond250, time.Millisecond * 250},
		{ClockAccuracySecond1, time.Second},
		{ClockAccuracySecond10, time.Second * 10},
		{ClockAccuracySecondGreater10, time.Second * 25}, // As per spec, this is just "> 10s"
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Accuracy %v", tc.input), func(t *testing.T) {
			if got := tc.input.Duration(); got != tc.expected {
				t.Errorf("For accuracy %v, expected %v, but got %v", tc.input, tc.expected, got)
			}
		})
	}
}
