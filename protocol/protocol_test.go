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
	"encoding/binary"
	"reflect"
	"testing"
)

func TestBytesTo(t *testing.T) {
	packet := &SyncDelayReq{
		Header: Header{
			SdoIDAndMsgType:     NewSdoIDAndMsgType(MessageSync, 1),
			Version:             MajorVersion,
			MessageLength:       44,
			DomainNumber:        0,
			MinorSdoID:          0,
			FlagField:           0,
			CorrectionField:     0,
			MessageTypeSpecific: 0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			SequenceID:         116,
			ControlField:       0,
			LogMessageInterval: 0,
		},
		SyncDelayReqBody: SyncDelayReqBody{
			OriginTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5a},
				Nanoseconds: 174389936,
			},
		},
	}

	b, err := Bytes(packet)
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	t.Run("buffer too small", func(t *testing.T) {
		buf := make([]byte, 10)
		_, err := BytesTo(packet, buf)
		if err == nil {
			t.Fatal("expected an error for a small buffer, but got nil")
		}
	})
	t.Run("just enough buffer", func(t *testing.T) {
		buf := make([]byte, len(b))
		l, err := BytesTo(packet, buf)
		if err != nil {
			t.Fatalf("BytesTo() failed: %v", err)
		}
		if l != len(b) {
			t.Errorf("expected length %d, got %d", len(b), l)
		}
		if !reflect.DeepEqual(b, buf) {
			t.Errorf("buffer content mismatch")
		}
	})
	t.Run("very big buffer", func(t *testing.T) {
		buf := make([]byte, len(b)+1000)
		l, err := BytesTo(packet, buf)
		if err != nil {
			t.Fatalf("BytesTo() failed: %v", err)
		}
		if l != len(b) {
			t.Errorf("expected length %d, got %d", len(b), l)
		}
		if !reflect.DeepEqual(b, buf[:l]) {
			t.Errorf("buffer content mismatch")
		}
	})
}

func TestParseSync(t *testing.T) {
	raw := []uint8{
		0x10, 0x02, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x63, 0xff,
		0xff, 0x00, 0x09, 0xba, 0x00, 0x01, 0x00, 0x74,
		0x00, 0x00, 0x00, 0x00, 0x45, 0xb1, 0x11, 0x5a,
		0x0a, 0x64, 0xfa, 0xb0, 0x00, 0x00,
	}
	packet := new(SyncDelayReq)
	err := FromBytes(raw, packet)
	if err != nil {
		t.Fatalf("FromBytes() error = %v", err)
	}
	want := SyncDelayReq{
		Header: Header{
			SdoIDAndMsgType:     NewSdoIDAndMsgType(MessageSync, 1),
			Version:             MajorVersion,
			MessageLength:       44,
			DomainNumber:        0,
			MinorSdoID:          0,
			FlagField:           0,
			CorrectionField:     0,
			MessageTypeSpecific: 0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			SequenceID:         116,
			ControlField:       0,
			LogMessageInterval: 0,
		},
		SyncDelayReqBody: SyncDelayReqBody{
			OriginTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5a},
				Nanoseconds: 174389936,
			},
		},
	}
	if !reflect.DeepEqual(want, *packet) {
		t.Errorf("got %+v, want %+v", *packet, want)
	}
	b, err := Bytes(packet)
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}
	if !reflect.DeepEqual(raw, b) {
		t.Errorf("got %v, want %v", b, raw)
	}

	// test generic DecodePacket as well
	pp, err := DecodePacket(raw)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	if !reflect.DeepEqual(&want, pp) {
		t.Errorf("got %v, want %v", pp, &want)
	}
}

func TestParseFollowup(t *testing.T) {
	raw := []uint8{
		0x8, 0x2, 0x0, 0x2c, 0x0, 0x0, 0x4, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x1, 0x0, 0x0, 0x2, 0x0, 0x0,
		0x0, 0x45, 0xb1, 0x11, 0x5e, 0x4, 0x5d, 0xd2, 0x6e, 0x0, 0x0,
	}
	packet := new(FollowUp)
	err := FromBytes(raw, packet)
	if err != nil {
		t.Fatalf("FromBytes() error = %v", err)
	}
	want := FollowUp{
		Header: Header{
			SdoIDAndMsgType: NewSdoIDAndMsgType(MessageFollowUp, 0),
			Version:         MajorVersion,
			MessageLength:   uint16(binary.Size(FollowUp{})), // #nosec:G115
			DomainNumber:    0,
			FlagField:       FlagUnicast,
			SequenceID:      0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			LogMessageInterval: 0,
			ControlField:       2,
		},
		FollowUpBody: FollowUpBody{
			PreciseOriginTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5e},
				Nanoseconds: 73257582,
			},
		},
	}
	if !reflect.DeepEqual(want, *packet) {
		t.Errorf("got %+v, want %+v", *packet, want)
	}
	b, err := Bytes(packet)
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}
	if !reflect.DeepEqual(raw, b) {
		t.Errorf("got %v, want %v", b, raw)
	}

	// test generic DecodePacket as well
	pp, err := DecodePacket(raw)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	if !reflect.DeepEqual(&want, pp) {
		t.Errorf("got %v, want %v", pp, &want)
	}
}

func TestParseAnnounce(t *testing.T) {
	raw := []uint8{
		0xb, 0x2, 0x0, 0x40, 0x0, 0x0, 0x4, 0x8, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x1, 0x0, 0x0, 0x5, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x6, 0x21, 0x59, 0xe0,
		0x80, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x0, 0x20, 0x0, 0x0,
	}
	packet := new(Announce)
	err := FromBytes(raw, packet)
	if err != nil {
		t.Fatalf("FromBytes() error = %v", err)
	}
	want := Announce{
		Header: Header{
			SdoIDAndMsgType: NewSdoIDAndMsgType(MessageAnnounce, 0),
			Version:         MajorVersion,
			MessageLength:   64,
			DomainNumber:    0,
			FlagField:       FlagUnicast | FlagPTPTimescale,
			SequenceID:      0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			LogMessageInterval: 0,
			ControlField:       5,
		},
		AnnounceBody: AnnounceBody{
			CurrentUTCOffset:     0,
			Reserved:             0,
			GrandmasterPriority1: 128,
			GrandmasterClockQuality: ClockQuality{
				ClockClass:              6,
				ClockAccuracy:           33, // 0x21 - Time Accurate within 100ns
				OffsetScaledLogVariance: 23008,
			},
			GrandmasterPriority2: 128,
			GrandmasterIdentity:  36138748164966842,
			StepsRemoved:         0,
			TimeSource:           TimeSourceGNSS,
		},
	}
	if !reflect.DeepEqual(want, *packet) {
		t.Errorf("got %+v, want %+v", *packet, want)
	}
	b, err := Bytes(packet)
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}
	if !reflect.DeepEqual(raw, b) {
		t.Errorf("got %v, want %v", b, raw)
	}

	// test generic DecodePacket as well
	pp, err := DecodePacket(raw)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	if !reflect.DeepEqual(&want, pp) {
		t.Errorf("got %v, want %v", pp, &want)
	}
}

func TestParseDelayResp(t *testing.T) {
	raw := []uint8{
		0x9, 0x2, 0x0, 0x36, 0x0, 0x0, 0x4, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x1, 0x0, 0xa, 0x3, 0x7f,
		0x0, 0x0, 0x45, 0xb1, 0x11, 0x5e, 0x4, 0x5d,
		0xd2, 0x6e, 0xb8, 0x59, 0x9f, 0xff, 0xfe,
		0x55, 0xaf, 0x4e, 0x0, 0x1, 0x0, 0x0,
	}
	packet := new(DelayResp)
	err := FromBytes(raw, packet)
	if err != nil {
		t.Fatalf("FromBytes() error = %v", err)
	}
	want := DelayResp{
		Header: Header{
			SdoIDAndMsgType: NewSdoIDAndMsgType(MessageDelayResp, 0),
			Version:         MajorVersion,
			MessageLength:   uint16(binary.Size(DelayResp{})), // #nosec:G115
			DomainNumber:    0,
			FlagField:       FlagUnicast,
			SequenceID:      10,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			LogMessageInterval: 0x7f,
			ControlField:       3,
			CorrectionField:    0,
		},
		DelayRespBody: DelayRespBody{
			ReceiveTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5e},
				Nanoseconds: 73257582,
			},
			RequestingPortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 13283824497738493774,
			},
		},
	}
	if !reflect.DeepEqual(want, *packet) {
		t.Errorf("got %+v, want %+v", *packet, want)
	}
	b, err := Bytes(packet)
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}
	if !reflect.DeepEqual(raw, b) {
		t.Errorf("got %v, want %v", b, raw)
	}

	// test generic DecodePacket as well
	pp, err := DecodePacket(raw)
	if err != nil {
		t.Fatalf("DecodePacket() error = %v", err)
	}
	if !reflect.DeepEqual(&want, pp) {
		t.Errorf("got %v, want %v", pp, &want)
	}
}

func TestFoundFuzzResults(t *testing.T) {
	allBytes := [][]byte{
		[]byte("00\x0000000000000000000000000000000000000000000\x00\x04\x00\x06000000"),
		[]byte("00\x00A0000000000000000000000000000000000000000\x00\x04\x00\x06000000\x00\x04\x00\x06000000000"),
	}
	for _, b := range allBytes {
		packet, err := DecodePacket(b)
		if err != nil {
			t.Fatalf("DecodePacket() error = %v", err)
		}
		bb, err := Bytes(packet)
		if err != nil {
			t.Fatalf("Bytes() failed unexpectedly: %v", err)
		}
		// ignore last 2 bytes as they are only for ipv6 checksums
		l := len(bb)
		if !reflect.DeepEqual(b[:l-2], bb[:l-2]) {
			t.Errorf("we expect binary form of packet %v %+v to be equal to original", packet.MessageType(), packet)
		}
	}
}

func BenchmarkReadSyncDelay(b *testing.B) {
	raw := []byte{1, 18, 0, 50, 0, 0, 36, 0, 0, 0, 0, 0, 6, 32, 0, 2, 0, 0, 0, 0, 184, 206, 246, 255, 254, 68, 148, 144, 0, 1, 149, 17, 0, 127, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 7, 0, 2, 16, 146, 0, 0}
	p := &SyncDelayReq{}
	for n := 0; n < b.N; n++ {
		_ = p.UnmarshalBinary(raw)
	}
}

func BenchmarkWriteSync(b *testing.B) {
	p := &SyncDelayReq{
		Header: Header{
			SdoIDAndMsgType:     NewSdoIDAndMsgType(MessageSync, 1),
			Version:             MajorVersion,
			MessageLength:       44,
			DomainNumber:        0,
			MinorSdoID:          0,
			FlagField:           0,
			CorrectionField:     0,
			MessageTypeSpecific: 0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			SequenceID:         116,
			ControlField:       0,
			LogMessageInterval: 0,
		},
		SyncDelayReqBody: SyncDelayReqBody{
			OriginTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5a},
				Nanoseconds: 174389936,
			},
		},
	}
	buf := make([]byte, 64)
	for n := 0; n < b.N; n++ {
		_, _ = BytesTo(p, buf)
	}
}

func BenchmarkReadAnnounce(b *testing.B) {
	raw := []uint8{
		0xb, 0x2, 0x0, 0x40, 0x0, 0x0, 0x4, 0x8, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x1, 0x0, 0x0, 0x5, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x6, 0x21, 0x59, 0xe0,
		0x80, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x0, 0x20, 0x0, 0x0,
	}
	p := &Announce{}
	for n := 0; n < b.N; n++ {
		_ = p.UnmarshalBinary(raw)
	}
}

func BenchmarkReadAnnouncePathTrace(b *testing.B) {
	raw := []uint8("\x0b\x12\x00\x4c\x00\x00\x04\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x08\xc0\xeb\xff\xfe\x63\x7a\x4e\x00\x01\x00\x00\x05\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x25\x00\x80\xf8\xfe\xff\xff\x80\x08\xc0\xeb\xff\xfe\x63\x7a\x4e\x00\x00\xa0\x00\x08\x00\x08\x08\xc0\xeb\xff\xfe\x63\x7a\x4e\x00\x00")
	p := &Announce{}
	for n := 0; n < b.N; n++ {
		_ = p.UnmarshalBinary(raw)
	}
}

func BenchmarkWriteAnnounce(b *testing.B) {
	p := &Announce{
		Header: Header{
			SdoIDAndMsgType: NewSdoIDAndMsgType(MessageAnnounce, 0),
			Version:         MajorVersion,
			MessageLength:   64,
			DomainNumber:    0,
			FlagField:       FlagUnicast | FlagPTPTimescale,
			SequenceID:      0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			LogMessageInterval: 0,
			ControlField:       5,
		},
		AnnounceBody: AnnounceBody{
			CurrentUTCOffset:     0,
			Reserved:             0,
			GrandmasterPriority1: 128,
			GrandmasterClockQuality: ClockQuality{
				ClockClass:              6,
				ClockAccuracy:           33, // 0x21 - Time Accurate within 100ns
				OffsetScaledLogVariance: 23008,
			},
			GrandmasterPriority2: 128,
			GrandmasterIdentity:  36138748164966842,
			StepsRemoved:         0,
			TimeSource:           TimeSourceGNSS,
		},
	}
	buf := make([]byte, 66)
	for n := 0; n < b.N; n++ {
		_, _ = BytesTo(p, buf)
	}
}

func BenchmarkWriteFollowup(b *testing.B) {
	p := &FollowUp{
		Header: Header{
			SdoIDAndMsgType: NewSdoIDAndMsgType(MessageFollowUp, 0),
			Version:         MajorVersion,
			MessageLength:   uint16(binary.Size(FollowUp{})), // #nosec:G115
			DomainNumber:    0,
			FlagField:       FlagUnicast,
			SequenceID:      0,
			SourcePortIdentity: PortIdentity{
				PortNumber:    1,
				ClockIdentity: 36138748164966842,
			},
			LogMessageInterval: 0,
			ControlField:       2,
		},
		FollowUpBody: FollowUpBody{
			PreciseOriginTimestamp: Timestamp{
				Seconds:     [6]byte{0x0, 0x00, 0x45, 0xb1, 0x11, 0x5e},
				Nanoseconds: 73257582,
			},
		},
	}
	buf := make([]byte, 64)
	for n := 0; n < b.N; n++ {
		_, _ = BytesTo(p, buf)
	}
}

func FuzzDecodePacket(f *testing.F) {
	delayResp := []uint8{
		0x9, 0x2, 0x0, 0x36, 0x0, 0x0, 0x4, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x80, 0x63, 0xff, 0xff, 0x0,
		0x9, 0xba, 0x0, 0x1, 0x0, 0xa, 0x3, 0x7f,
		0x0, 0x0, 0x45, 0xb1, 0x11, 0x5e, 0x4, 0x5d,
		0xd2, 0x6e, 0xb8, 0x59, 0x9f, 0xff, 0xfe,
		0x55, 0xaf, 0x4e, 0x0, 0x1, 0x0, 0x0,
	}
	signalingGrantUnicast := []uint8{0x0c, 0x02, 0x00, 0x38, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xe4, 0x1d, 0x2d, 0xff, 0xfe, 0xbb, 0x64, 0x60, 0x00,
		0x01, 0x1d, 0xc4, 0x05, 0x7f, 0x48, 0x57, 0xdd, 0xff,
		0xfe, 0x08, 0x64, 0x88, 0x00, 0x01, 0x00, 0x05, 0x00,
		0x08, 0xb0, 0x01, 0x00, 0x00, 0x00, 0x3c, 0x00, 0x01,
		0x00, 0x00,
	}
	for _, seed := range [][]byte{{}, {0}, {9}, delayResp, signalingGrantUnicast} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		packet, err := DecodePacket(b)
		// check that marshalling works
		if err == nil {
			// special case for TLVs that have variable length
			// which may have padding and thus marshalling produces different results
			switch packet.MessageType() {
			case MessageSignaling:
				m := packet.(*Signaling)
				// ignore Signaling with PathTrace or AlternateTimeOffsetIndicator TLVs
				for _, tlv := range m.TLVs {
					if tlv.Type() == TLVPathTrace {
						return
					}
					if tlv.Type() == TLVAlternateTimeOffsetIndicator {
						return
					}
				}
			// Announce msg can have TracePath TLV which also has PTPText
			case MessageAnnounce:
				m := packet.(*Announce)
				// ignore Announce with PathTrace or AlternateTimeOffsetIndicator TLVs
				for _, tlv := range m.TLVs {
					if tlv.Type() == TLVPathTrace {
						return
					}
					if tlv.Type() == TLVAlternateTimeOffsetIndicator {
						return
					}
				}
			case MessageSync, MessageDelayReq:
				m := packet.(*SyncDelayReq)
				// ignore Sync/DelayReq with GrantUnicastTransmissionTLV, TLVPathTrace or TLVAlternateTimeOffsetIndicator TLVs
				for _, tlv := range m.TLVs {
					if tlv.Type() == TLVGrantUnicastTransmission {
						return
					}
					if tlv.Type() == TLVAlternateTimeOffsetIndicator {
						return
					}
					if tlv.Type() == TLVPathTrace {
						return
					}
				}
			}
			bb, err := Bytes(packet)
			if err != nil {
				t.Fatalf("Bytes() failed unexpectedly: %v", err)
			}
			// ignore last 2 bytes as they are only for ipv6 checksums
			l := len(bb)
			if !reflect.DeepEqual(b[:l-2], bb[:l-2]) {
				t.Errorf("we expect binary form of packet %v %+v to be equal to original", packet.MessageType(), packet)
			}
		} else {
			if packet != nil {
				t.Fatalf("packet should be nil on error")
			}
		}
	})
}
