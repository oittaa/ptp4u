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

package server

import (
	"context"
	"encoding/binary"
	"net"
	"reflect"
	"testing"
	"time"

	ptp "github.com/oittaa/ptp4u/protocol"
	"github.com/oittaa/ptp4u/timestamp"
)

func TestRunning(t *testing.T) {
	sc := SubscriptionClient{}
	// Initially subscription is not running (expire time is in the past)
	if !sc.Expired() {
		t.Fatal("new subscription should be expired")
	}

	// Add proper actual expiration time subscription
	sc.SetExpire(time.Now().Add(1 * time.Second))
	if sc.Expired() {
		t.Fatal("subscription should not be expired")
	}

	// Check running status
	if sc.Running() {
		t.Fatal("subscription should not be running initially")
	}
	sc.setRunning(true)
	if !sc.Running() {
		t.Fatal("subscription should be running")
	}
}

func TestSubscriptionStart(t *testing.T) {
	w := &sendWorker{}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	interval := 1 * time.Minute
	expire := time.Now().Add(1 * time.Minute)
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, nil, ptp.MessageAnnounce, c, interval, expire)
	sc.SetGclisa(sa)

	go sc.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	if sc.Expired() {
		t.Fatal("subscription should not be expired")
	}
	if !sc.Running() {
		t.Fatal("subscription should be running")
	}
}

func TestSubscriptionExpire(t *testing.T) {
	w := &sendWorker{
		signalingQueue: make(chan *SubscriptionClient, 100),
	}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	interval := 10 * time.Millisecond
	expire := time.Now().Add(200 * time.Millisecond)
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageDelayResp, c, interval, expire)

	go sc.Start(context.Background())
	time.Sleep(100 * time.Millisecond)

	if sc.Expired() {
		t.Fatal("subscription should not be expired yet")
	}
	if !sc.Running() {
		t.Fatal("subscription should be running")
	}

	// Wait to expire
	time.Sleep(150 * time.Millisecond)
	if !sc.Expired() {
		t.Fatal("subscription should be expired")
	}
	if sc.Running() {
		t.Fatal("subscription should not be running after expiration")
	}
}

func TestSubscriptionStop(t *testing.T) {
	w := &sendWorker{
		queue:          make(chan *SubscriptionClient, 100),
		signalingQueue: make(chan *SubscriptionClient, 100),
	}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	interval := 32 * time.Second
	expire := time.Now().Add(1 * time.Minute)
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, interval, expire)

	go sc.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	if sc.Expired() {
		t.Fatal("subscription should not be expired")
	}
	if !sc.Running() {
		t.Fatal("subscription should be running")
	}

	sc.Stop()
	time.Sleep(100 * time.Millisecond)

	if !sc.Expired() {
		t.Fatal("subscription should be expired after stop")
	}
	if sc.Running() {
		t.Fatal("subscription should not be running after stop")
	}

	// No matter how many times we run stop we should not lock
	sc.Stop()
	sc.Stop()
	sc.Stop()

	if len(w.signalingQueue) != 1 {
		t.Fatalf("expected 1 item in signaling queue, got %d", len(w.signalingQueue))
	}
	s := <-w.signalingQueue
	tlv, ok := s.signaling.TLVs[0].(*ptp.CancelUnicastTransmissionTLV)
	if !ok {
		t.Fatalf("could not assert TLV type")
	}
	if tlv.TLVType != ptp.TLVCancelUnicastTransmission {
		t.Fatalf("unexpected tlv type, got %v, want %v", tlv.TLVType, ptp.TLVCancelUnicastTransmission)
	}
	expectedLen := uint16(binary.Size(ptp.Header{}) + binary.Size(ptp.PortIdentity{}) + binary.Size(ptp.CancelUnicastTransmissionTLV{})) //#nosec:G115
	if s.signaling.MessageLength != expectedLen {
		t.Fatalf("unexpected message length, got %d, want %d", s.signaling.MessageLength, expectedLen)
	}
}

func TestSubscriptionEnd(t *testing.T) {
	w := &sendWorker{
		signalingQueue: make(chan *SubscriptionClient, 100),
	}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	interval := 10 * time.Millisecond
	expire := time.Now().Add(300 * time.Millisecond)
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageDelayResp, c, interval, expire)

	ctx, cancel := context.WithCancel(context.Background())
	go sc.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	if !sc.Running() {
		t.Fatal("subscription should be running")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	if sc.Running() {
		t.Fatal("subscription should not be running after context cancellation")
	}
}

func TestSubscriptionflags(t *testing.T) {
	w := &sendWorker{}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	sc.UpdateSync()
	sc.UpdateFollowup(time.Now())
	sc.UpdateAnnounce()
	if sc.Sync().FlagField != ptp.FlagUnicast|ptp.FlagTwoStep {
		t.Errorf("unexpected sync flags: got %v, want %v", sc.Sync().FlagField, ptp.FlagUnicast|ptp.FlagTwoStep)
	}
	if sc.Followup().FlagField != ptp.FlagUnicast {
		t.Errorf("unexpected followup flags: got %v, want %v", sc.Followup().FlagField, ptp.FlagUnicast)
	}
	if sc.Announce().FlagField != ptp.FlagUnicast|ptp.FlagPTPTimescale {
		t.Errorf("unexpected announce flags: got %v, want %v", sc.Announce().FlagField, ptp.FlagUnicast|ptp.FlagPTPTimescale)
	}
}

func TestSyncPacket(t *testing.T) {
	sequenceID := uint16(42)
	domainNumber := uint8(13)

	w := &sendWorker{}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})
	sc.sequenceID = sequenceID

	sc.initSync()
	sc.IncSequenceID()
	sc.UpdateSync()
	if sc.Sync().MessageLength != 44 {
		t.Errorf("unexpected sync packet length: got %d, want 44", sc.Sync().MessageLength)
	}
	if sc.Sync().SequenceID != sequenceID+1 {
		t.Errorf("unexpected sync sequence id: got %d, want %d", sc.Sync().SequenceID, sequenceID+1)
	}
	if sc.Sync().DomainNumber != domainNumber {
		t.Errorf("unexpected sync domain number: got %d, want %d", sc.Sync().DomainNumber, domainNumber)
	}
}

func TestSyncDelayReqPacket(t *testing.T) {
	sequenceID := uint16(42)
	domainNumber := uint8(13)
	received := time.Now()

	w := &sendWorker{}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})
	sc.sequenceID = sequenceID

	sc.initSync()
	sc.UpdateSyncDelayReq(received, sequenceID)
	if sc.Sync().MessageLength != 44 {
		t.Errorf("unexpected sync packet length: got %d, want 44", sc.Sync().MessageLength)
	}
	if sc.Sync().SequenceID != sequenceID {
		t.Errorf("unexpected sync sequence id: got %d, want %d", sc.Sync().SequenceID, sequenceID)
	}
	if sc.Sync().DomainNumber != domainNumber {
		t.Errorf("unexpected sync domain number: got %d, want %d", sc.Sync().DomainNumber, domainNumber)
	}
	if !reflect.DeepEqual(sc.Sync().OriginTimestamp, ptp.NewTimestamp(received)) {
		t.Errorf("unexpected sync origin timestamp: got %v, want %v", sc.Sync().OriginTimestamp, ptp.NewTimestamp(received))
	}
}

func TestFollowupPacket(t *testing.T) {
	sequenceID := uint16(42)
	now := time.Now()
	interval := 3 * time.Second
	domainNumber := uint8(13)

	w := &sendWorker{}

	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})
	sc.sequenceID = sequenceID
	sc.SetInterval(interval)

	i, err := ptp.NewLogInterval(interval)
	if err != nil {
		t.Fatalf("unexpected error from NewLogInterval: %v", err)
	}

	sc.initFollowup()
	sc.IncSequenceID()
	sc.UpdateFollowup(now)
	if sc.Followup().MessageLength != 44 {
		t.Errorf("unexpected followup packet length: got %d, want 44", sc.Followup().MessageLength)
	}
	if sc.Followup().SequenceID != sequenceID+1 {
		t.Errorf("unexpected followup sequence id: got %d, want %d", sc.Followup().SequenceID, sequenceID+1)
	}
	if sc.Followup().LogMessageInterval != i {
		t.Errorf("unexpected followup log interval: got %v, want %v", sc.Followup().LogMessageInterval, i)
	}
	if sc.Followup().PreciseOriginTimestamp.Time().Unix() != now.Unix() {
		t.Errorf("unexpected followup origin timestamp: got %v, want %v", sc.Followup().PreciseOriginTimestamp.Time(), now)
	}
	if sc.Followup().DomainNumber != domainNumber {
		t.Errorf("unexpected followup domain number: got %d, want %d", sc.Followup().DomainNumber, domainNumber)
	}
}

func TestAnnouncePacket(t *testing.T) {
	UTCOffset := 3 * time.Second
	sequenceID := uint16(42)
	interval := 3 * time.Second
	clockClass := ptp.ClockClass7
	clockAccuracy := ptp.ClockAccuracyMicrosecond1
	domainNumber := uint8(13)

	w := &sendWorker{}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		DynamicConfig: DynamicConfig{
			ClockClass:    clockClass,
			ClockAccuracy: clockAccuracy,
			UTCOffset:     UTCOffset,
		},
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})
	sc.sequenceID = sequenceID
	sc.SetInterval(interval)

	i, err := ptp.NewLogInterval(interval)
	if err != nil {
		t.Fatalf("unexpected error from NewLogInterval: %v", err)
	}

	sp := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}

	sc.initAnnounce()
	sc.IncSequenceID()
	sc.UpdateAnnounce()

	announce := sc.Announce()
	if announce.MessageLength != 64 {
		t.Errorf("unexpected announce packet length: got %d, want 64", announce.MessageLength)
	}
	if announce.SequenceID != sequenceID+1 {
		t.Errorf("unexpected announce sequence id: got %d, want %d", announce.SequenceID, sequenceID+1)
	}
	if !reflect.DeepEqual(announce.SourcePortIdentity, sp) {
		t.Errorf("unexpected announce source port identity: got %v, want %v", announce.SourcePortIdentity, sp)
	}
	if announce.LogMessageInterval != i {
		t.Errorf("unexpected announce log interval: got %v, want %v", announce.LogMessageInterval, i)
	}
	if announce.GrandmasterClockQuality.ClockClass != ptp.ClockClass7 {
		t.Errorf("unexpected announce clock class: got %v, want %v", announce.GrandmasterClockQuality.ClockClass, ptp.ClockClass7)
	}
	if announce.GrandmasterClockQuality.ClockAccuracy != ptp.ClockAccuracyMicrosecond1 {
		t.Errorf("unexpected announce clock accuracy: got %v, want %v", announce.GrandmasterClockQuality.ClockAccuracy, ptp.ClockAccuracyMicrosecond1)
	}
	if announce.CurrentUTCOffset != int16(UTCOffset.Seconds()) {
		t.Errorf("unexpected announce utc offset: got %v, want %v", announce.CurrentUTCOffset, int16(UTCOffset.Seconds()))
	}
	if announce.DomainNumber != domainNumber {
		t.Errorf("unexpected announce domain number: got %d, want %d", announce.DomainNumber, domainNumber)
	}
}

func TestAnnounceDelayReqPacket(t *testing.T) {
	UTCOffset := 3 * time.Second
	sequenceID := uint16(42)
	clockClass := ptp.ClockClass7
	clockAccuracy := ptp.ClockAccuracyMicrosecond1
	domainNumber := uint8(13)
	correctionField := ptp.NewCorrection(100500)
	now := time.Now()
	transmit := ptp.NewTimestamp(now)

	w := &sendWorker{}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		DynamicConfig: DynamicConfig{
			ClockClass:    clockClass,
			ClockAccuracy: clockAccuracy,
			UTCOffset:     UTCOffset,
		},
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	sp := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}

	sc.initAnnounce()
	sc.UpdateAnnounceDelayReq(correctionField, sequenceID)
	sc.UpdateAnnounceFollowUp(now)

	announce := sc.Announce()
	if announce.MessageLength != 64 {
		t.Errorf("unexpected announce packet length: got %d, want 64", announce.MessageLength)
	}
	if announce.SequenceID != sequenceID {
		t.Errorf("unexpected announce sequence id: got %d, want %d", announce.SequenceID, sequenceID)
	}
	if !reflect.DeepEqual(announce.SourcePortIdentity, sp) {
		t.Errorf("unexpected announce source port identity: got %v, want %v", announce.SourcePortIdentity, sp)
	}
	if announce.GrandmasterClockQuality.ClockClass != ptp.ClockClass7 {
		t.Errorf("unexpected announce clock class: got %v, want %v", announce.GrandmasterClockQuality.ClockClass, ptp.ClockClass7)
	}
	if announce.GrandmasterClockQuality.ClockAccuracy != ptp.ClockAccuracyMicrosecond1 {
		t.Errorf("unexpected announce clock accuracy: got %v, want %v", announce.GrandmasterClockQuality.ClockAccuracy, ptp.ClockAccuracyMicrosecond1)
	}
	if announce.CurrentUTCOffset != int16(UTCOffset.Seconds()) {
		t.Errorf("unexpected announce utc offset: got %v, want %v", announce.CurrentUTCOffset, int16(UTCOffset.Seconds()))
	}
	if announce.DomainNumber != domainNumber {
		t.Errorf("unexpected announce domain number: got %d, want %d", announce.DomainNumber, domainNumber)
	}
	if !reflect.DeepEqual(announce.CorrectionField, correctionField) {
		t.Errorf("unexpected announce correction field: got %v, want %v", announce.CorrectionField, correctionField)
	}
	if !reflect.DeepEqual(announce.OriginTimestamp, transmit) {
		t.Errorf("unexpected announce origin timestamp: got %v, want %v", announce.OriginTimestamp, transmit)
	}
}

func TestDelayRespPacket(t *testing.T) {
	sequenceID := uint16(42)
	now := time.Now()
	domainNumber := uint8(13)

	w := &sendWorker{}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			DomainNumber: uint(domainNumber),
		},
	}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	sp := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}
	h := &ptp.Header{
		SequenceID:         sequenceID,
		CorrectionField:    ptp.NewCorrection(100500),
		SourcePortIdentity: sp,
	}

	sc.initDelayResp()
	sc.UpdateDelayResp(h, now)

	delayResp := sc.DelayResp()
	if delayResp.MessageLength != 54 {
		t.Errorf("unexpected delay_resp packet length: got %d, want 54", delayResp.MessageLength)
	}
	if delayResp.SequenceID != sequenceID {
		t.Errorf("unexpected delay_resp sequence id: got %d, want %d", delayResp.SequenceID, sequenceID)
	}
	if int(delayResp.CorrectionField.Nanoseconds()) != 100500 {
		t.Errorf("unexpected delay_resp correction field: got %f, want 100500", delayResp.CorrectionField.Nanoseconds())
	}
	if !reflect.DeepEqual(delayResp.SourcePortIdentity, sp) {
		t.Errorf("unexpected delay_resp source port identity: got %v, want %v", delayResp.SourcePortIdentity, sp)
	}
	if delayResp.DelayRespBody.ReceiveTimestamp.Time().Unix() != now.Unix() {
		t.Errorf("unexpected delay_resp receive timestamp: got %v, want %v", delayResp.ReceiveTimestamp.Time(), now)
	}
	if delayResp.FlagField != ptp.FlagUnicast {
		t.Errorf("unexpected delay_resp flags: got %v, want %v", delayResp.FlagField, ptp.FlagUnicast)
	}
	if delayResp.DomainNumber != domainNumber {
		t.Errorf("unexpected delay_resp domain number: got %d, want %d", delayResp.DomainNumber, domainNumber)
	}
}

func TestSignalingGrantPacket(t *testing.T) {
	interval := 3 * time.Second

	w := &sendWorker{}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})
	sg := &ptp.Signaling{}

	mt := ptp.NewUnicastMsgTypeAndFlags(ptp.MessageAnnounce, 0)
	i, err := ptp.NewLogInterval(interval)
	if err != nil {
		t.Fatalf("unexpected error from NewLogInterval: %v", err)
	}
	duration := uint32(3)

	tlv := &ptp.GrantUnicastTransmissionTLV{
		TLVHead: ptp.TLVHead{
			TLVType:     ptp.TLVGrantUnicastTransmission,
			LengthField: uint16(binary.Size(ptp.GrantUnicastTransmissionTLV{}) - binary.Size(ptp.TLVHead{})), //#nosec:G115
		},
		MsgTypeAndReserved:    mt,
		LogInterMessagePeriod: i,
		DurationField:         duration,
		Reserved:              0,
		Renewal:               1,
	}

	sc.initSignaling()
	sc.UpdateSignalingGrant(sg, mt, i, duration)

	if sc.Signaling().MessageLength != 56 {
		t.Errorf("unexpected signaling grant packet length: got %d, want 56", sc.Signaling().MessageLength)
	}
	if !reflect.DeepEqual(sc.Signaling().TLVs[0], tlv) {
		t.Errorf("unexpected signaling grant tlv:\ngot:  %+v\nwant: %+v", sc.Signaling().TLVs[0], tlv)
	}
}

func TestSignalingCancelPacket(t *testing.T) {
	w := &sendWorker{}
	c := &Config{clockIdentity: ptp.ClockIdentity(1234)}
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	sc.signaling.MessageLength = uint16(binary.Size(ptp.Header{}) + binary.Size(ptp.PortIdentity{}) + binary.Size(ptp.CancelUnicastTransmissionTLV{})) //#nosec:G115
	tlv := &ptp.CancelUnicastTransmissionTLV{
		TLVHead:         ptp.TLVHead{TLVType: ptp.TLVCancelUnicastTransmission, LengthField: uint16(binary.Size(ptp.CancelUnicastTransmissionTLV{}) - binary.Size(ptp.TLVHead{}))}, //#nosec:G115
		Reserved:        0,
		MsgTypeAndFlags: ptp.NewUnicastMsgTypeAndFlags(ptp.MessageAnnounce, 0),
	}

	sc.initSignaling()
	sc.UpdateSignalingCancel()

	if sc.Signaling().MessageLength != 50 {
		t.Errorf("unexpected signaling cancel packet length: got %d, want 50", sc.Signaling().MessageLength)
	}
	if !reflect.DeepEqual(sc.Signaling().TLVs[0], tlv) {
		t.Errorf("unexpected signaling cancel tlv:\ngot:  %+v\nwant: %+v", sc.Signaling().TLVs[0], tlv)
	}
}

func TestSendSignalingGrant(t *testing.T) {
	w := &sendWorker{
		signalingQueue: make(chan *SubscriptionClient, 10),
	}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			SendWorkers: 10,
		},
	}

	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	if len(w.signalingQueue) != 0 {
		t.Fatalf("signaling queue should be empty initially, got %d", len(w.signalingQueue))
	}
	sc.sendSignalingGrant(&ptp.Signaling{}, 0, 0, 0)
	if len(w.signalingQueue) != 1 {
		t.Fatalf("signaling queue should have 1 item, got %d", len(w.signalingQueue))
	}

	s := <-w.signalingQueue
	tlv, ok := s.signaling.TLVs[0].(*ptp.GrantUnicastTransmissionTLV)
	if !ok {
		t.Fatal("could not assert TLV type")
	}
	if tlv.TLVType != ptp.TLVGrantUnicastTransmission {
		t.Fatalf("unexpected tlv type, got %v, want %v", tlv.TLVType, ptp.TLVGrantUnicastTransmission)
	}
	expectedLen := uint16(binary.Size(ptp.Header{}) + binary.Size(ptp.PortIdentity{}) + binary.Size(ptp.GrantUnicastTransmissionTLV{})) //#nosec:G115
	if s.signaling.MessageLength != expectedLen {
		t.Fatalf("unexpected message length, got %d, want %d", s.signaling.MessageLength, expectedLen)
	}
}

func TestSendSignalingCancel(t *testing.T) {
	w := &sendWorker{
		signalingQueue: make(chan *SubscriptionClient, 10),
	}
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			SendWorkers: 10,
		},
	}

	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	sc := NewSubscriptionClient(w.queue, w.signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, time.Time{})

	if len(w.signalingQueue) != 0 {
		t.Fatalf("signaling queue should be empty initially, got %d", len(w.signalingQueue))
	}
	sc.sendSignalingCancel()
	if len(w.signalingQueue) != 1 {
		t.Fatalf("signaling queue should have 1 item, got %d", len(w.signalingQueue))
	}

	s := <-w.signalingQueue
	tlv, ok := s.signaling.TLVs[0].(*ptp.CancelUnicastTransmissionTLV)
	if !ok {
		t.Fatal("could not assert TLV type")
	}
	if tlv.TLVType != ptp.TLVCancelUnicastTransmission {
		t.Fatalf("unexpected tlv type, got %v, want %v", tlv.TLVType, ptp.TLVCancelUnicastTransmission)
	}
	expectedLen := uint16(binary.Size(ptp.Header{}) + binary.Size(ptp.PortIdentity{}) + binary.Size(ptp.CancelUnicastTransmissionTLV{})) //#nosec:G115
	if s.signaling.MessageLength != expectedLen {
		t.Fatalf("unexpected message length, got %d, want %d", s.signaling.MessageLength, expectedLen)
	}
}
