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
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	ptp "github.com/oittaa/ptp4u/protocol"
	"github.com/oittaa/ptp4u/stats"
	"github.com/oittaa/ptp4u/timestamp"

	"golang.org/x/sys/unix"
)

// checkClose is a test helper that closes the given resource and fails the
// test if the close operation returns an error.
func checkClose(t *testing.T, closer io.Closer) {
	// t.Helper() marks this function as a test helper.
	// When t.Errorf is called, the line number reported will be from the
	// calling function, not from inside checkClose.
	t.Helper()
	if err := closer.Close(); err != nil {
		t.Errorf("failed to close resource: %v", err)
	}
}

// checkRemove is a test helper that removes the given file and fails the
// test if the remove operation returns an error.
func checkRemove(t *testing.T, name string) {
	t.Helper()
	if err := os.Remove(name); err != nil {
		t.Errorf("failed to remove file %q: %v", name, err)
	}
}

// Helper function to check if a file exists
func fileExists(t *testing.T, filename string) {
	t.Helper()
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("file %q was expected to exist, but it doesn't", filename)
	}
}

// Helper function to check if a file does not exist
func noFileExists(t *testing.T, filename string) {
	t.Helper()
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("file %q was expected not to exist, but it does", filename)
	}
}

func TestFindWorker(t *testing.T) {
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			TimestampType: timestamp.SW,
			SendWorkers:   100,
		},
	}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
		sw:     make([]*sendWorker, c.SendWorkers),
	}

	for i := 0; i < s.Config.SendWorkers; i++ {
		s.sw[i] = newSendWorker(i, c, s.Stats)
	}

	clipi1 := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}

	clipi2 := ptp.PortIdentity{
		PortNumber:    2,
		ClockIdentity: ptp.ClockIdentity(1234),
	}

	clipi3 := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(5678),
	}

	expected1 := s.findWorker(clipi1, 0).id
	// Consistent across multiple calls
	if id := s.findWorker(clipi1, 0).id; id != expected1 {
		t.Errorf("expected worker id %d, got %d", expected1, id)
	}
	if id := s.findWorker(clipi1, 0).id; id != expected1 {
		t.Errorf("expected worker id %d, got %d", expected1, id)
	}
	if id := s.findWorker(clipi1, 0).id; id != expected1 {
		t.Errorf("expected worker id %d, got %d", expected1, id)
	}

	expected2 := s.findWorker(clipi2, 0).id
	if id := s.findWorker(clipi2, 0).id; id != expected2 {
		t.Errorf("expected worker id %d, got %d", expected2, id)
	}
	expected3 := s.findWorker(clipi3, 0).id
	if id := s.findWorker(clipi3, 0).id; id != expected3 {
		t.Errorf("expected worker id %d, got %d", expected3, id)
	}
	expected4 := s.findWorker(clipi1, 1).id
	if expected1 == expected2 && expected2 == expected3 && expected3 == expected4 {
		t.Errorf("expected at least one different worker id, got only %d with seed %v", expected1, seed)
	}
}

func TestStartEventListener(t *testing.T) {
	ptp.PortEvent = 0
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			TimestampType: timestamp.SW,
			SendWorkers:   10,
			RecvWorkers:   10,
			IP:            net.ParseIP("127.0.0.1"),
		},
	}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
		sw:     make([]*sendWorker, c.SendWorkers),
	}
	go s.startEventListener()
	// Give it a moment to start listening. If it fails, it will panic.
	time.Sleep(100 * time.Millisecond)
}

func TestStartGeneralListener(t *testing.T) {
	ptp.PortGeneral = 0
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			TimestampType: timestamp.SW,
			SendWorkers:   10,
			RecvWorkers:   10,
			IP:            net.ParseIP("127.0.0.1"),
		},
	}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
		sw:     make([]*sendWorker, c.SendWorkers),
	}
	go s.startGeneralListener()
	// Give it a moment to start listening. If it fails, it will panic.
	time.Sleep(100 * time.Millisecond)
}

func TestDrain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := Server{
		Stats:  stats.NewJSONStats(),
		ctx:    ctx,
		cancel: cancel,
	}

	if err := s.ctx.Err(); err != nil {
		t.Fatalf("expected no error initially, got %v", err)
	}
	s.Drain()
	if err := s.ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestUndrain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := Server{
		Stats:  stats.NewJSONStats(),
		ctx:    ctx,
		cancel: cancel,
	}

	s.Drain()
	if err := s.ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
	s.Undrain()
	if err := s.ctx.Err(); err != nil {
		t.Fatalf("expected no error after undrain, got %v", err)
	}
}

func TestHandleSighup(t *testing.T) {
	// Values from the config file below
	expectedDC := DynamicConfig{
		ClockAccuracy:  0x31,
		ClockClass:     1,
		DrainInterval:  2 * time.Second,
		MaxSubDuration: 3 * time.Hour,
		MetricInterval: 4 * time.Minute,
		MinSubInterval: 5 * time.Second,
		UTCOffset:      37 * time.Second,
	}

	c := &Config{
		StaticConfig: StaticConfig{
			SendWorkers: 2,
			QueueSize:   10,
		},
	}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
		sw:     make([]*sendWorker, c.SendWorkers),
	}
	clipi := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}

	s.sw[0] = newSendWorker(0, s.Config, s.Stats)
	s.sw[1] = newSendWorker(1, s.Config, s.Stats)
	sa := timestamp.IPToSockaddr(net.ParseIP("127.0.0.1"), 123)
	// Make sure subscriptions are not expired
	expire := time.Now().Add(time.Minute)
	scA := NewSubscriptionClient(s.sw[0].queue, s.sw[0].signalingQueue, sa, sa, ptp.MessageAnnounce, c, time.Second, expire)
	scS := NewSubscriptionClient(s.sw[1].queue, s.sw[1].signalingQueue, sa, sa, ptp.MessageSync, c, time.Second, expire)
	s.sw[0].RegisterSubscription(clipi, ptp.MessageAnnounce, scA)
	s.sw[1].RegisterSubscription(clipi, ptp.MessageSync, scS)

	go scA.Start(context.Background())
	go scS.Start(context.Background())
	time.Sleep(100 * time.Millisecond)

	if l := len(s.sw[0].queue); l != 1 {
		t.Errorf("expected queue length 1 for worker 0, got %d", l)
	}
	if l := len(s.sw[1].queue); l != 1 {
		t.Errorf("expected queue length 1 for worker 1, got %d", l)
	}

	cfg, err := os.CreateTemp("", "ptp4u-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer checkRemove(t, cfg.Name())

	config := `clockaccuracy: 0x31
clockclass: 1
draininterval: "2s"
maxsubduration: "3h"
metricinterval: "4m"
minsubinterval: "5s"
utcoffset: "37s"
`
	_, err = cfg.WriteString(config)
	if err != nil {
		t.Fatalf("failed to write to temp config file: %v", err)
	}
	checkClose(t, cfg)

	c.ConfigFile = cfg.Name()

	go s.handleSighup()
	time.Sleep(100 * time.Millisecond)

	err = unix.Kill(unix.Getpid(), unix.SIGHUP)
	if err != nil {
		t.Fatalf("failed to send SIGHUP: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	dcMux.Lock()
	if !reflect.DeepEqual(expectedDC, c.DynamicConfig) {
		t.Errorf("config mismatch:\ngot:  %+v\nwant: %+v", c.DynamicConfig, expectedDC)
	}
	dcMux.Unlock()

	// Queues should still have the items
	if l := len(s.sw[0].queue); l != 1 {
		t.Errorf("expected queue length 1 for worker 0 after SIGHUP, got %d", l)
	}
	if l := len(s.sw[1].queue); l != 1 {
		t.Errorf("expected queue length 1 for worker 1 after SIGHUP, got %d", l)
	}
}

func TestHandleSigterm(t *testing.T) {
	cfg, err := os.CreateTemp("", "ptp4u-pid-*.pid")
	if err != nil {
		t.Fatalf("failed to create temp pid file: %v", err)
	}
	pidFileName := cfg.Name()
	checkClose(t, cfg)
	_ = os.Remove(pidFileName) // Ensure it doesn't exist before we start
	noFileExists(t, pidFileName)

	c := &Config{StaticConfig: StaticConfig{PidFile: pidFileName}}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
	}

	err = c.CreatePidFile()
	if err != nil {
		t.Fatalf("failed to create pid file: %v", err)
	}
	fileExists(t, c.PidFile)

	// Delayed SIGTERM
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = unix.Kill(unix.Getpid(), unix.SIGTERM)
	}()

	// This should block until the signal is received, then clean up and exit the function.
	s.handleSigterm()

	// Check that the pid file was removed
	noFileExists(t, c.PidFile)
}

func BenchmarkFindWorker(b *testing.B) {
	c := &Config{
		clockIdentity: ptp.ClockIdentity(1234),
		StaticConfig: StaticConfig{
			TimestampType: timestamp.SW,
			SendWorkers:   100,
		},
	}
	s := Server{
		Config: c,
		Stats:  stats.NewJSONStats(),
		sw:     make([]*sendWorker, c.SendWorkers),
	}

	for i := 0; i < s.Config.SendWorkers; i++ {
		s.sw[i] = newSendWorker(i, c, s.Stats)
	}

	clipi1 := ptp.PortIdentity{
		PortNumber:    1,
		ClockIdentity: ptp.ClockIdentity(1234),
	}
	for n := 0; n < b.N; n++ {
		_ = s.findWorker(clipi1, 0)
	}
}
