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

package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"

	ptp "github.com/oittaa/ptp4u/protocol"
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

func getFreePort(t *testing.T) (int, error) {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer checkClose(t, l)
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestJSONStatsReset(t *testing.T) {
	stats := JSONStats{}
	stats.subscriptions.init()
	stats.rxSignalingGrant.init()
	stats.rxSignalingCancel.init()
	stats.workerQueue.init()

	stats.IncSubscription(ptp.MessageAnnounce)
	stats.IncRXSignalingGrant(ptp.MessageSync)
	stats.IncRXSignalingCancel(ptp.MessageSync)
	stats.SetMaxWorkerQueue(10, 42)

	stats.Reset()
	if val := stats.subscriptions.load(int(ptp.MessageAnnounce)); val != 0 {
		t.Errorf("expected subscriptions to be 0, got %d", val)
	}
	if val := stats.rxSignalingGrant.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected rxSignalingGrant to be 0, got %d", val)
	}
	if val := stats.rxSignalingCancel.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected rxSignalingCancel to be 0, got %d", val)
	}
	if val := stats.workerQueue.load(10); val != 0 {
		t.Errorf("expected workerQueue to be 0, got %d", val)
	}
}

func TestJSONStatsAnnounceSubscription(t *testing.T) {
	stats := NewJSONStats()

	stats.IncSubscription(ptp.MessageAnnounce)
	if val := stats.subscriptions.load(int(ptp.MessageAnnounce)); val != 1 {
		t.Errorf("expected subscriptions to be 1, got %d", val)
	}

	stats.DecSubscription(ptp.MessageAnnounce)
	if val := stats.subscriptions.load(int(ptp.MessageAnnounce)); val != 0 {
		t.Errorf("expected subscriptions to be 0, got %d", val)
	}
}

func TestJSONStatsSyncSubscription(t *testing.T) {
	stats := NewJSONStats()

	stats.IncSubscription(ptp.MessageSync)
	if val := stats.subscriptions.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected subscriptions to be 1, got %d", val)
	}

	stats.DecSubscription(ptp.MessageSync)
	if val := stats.subscriptions.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected subscriptions to be 0, got %d", val)
	}
}

func TestJSONStatsRX(t *testing.T) {
	stats := NewJSONStats()

	stats.IncRX(ptp.MessageSync)
	if val := stats.rx.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected rx to be 1, got %d", val)
	}

	stats.DecRX(ptp.MessageSync)
	if val := stats.rx.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected rx to be 0, got %d", val)
	}
}

func TestJSONStatsTX(t *testing.T) {
	stats := NewJSONStats()

	stats.IncTX(ptp.MessageSync)
	if val := stats.tx.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected tx to be 1, got %d", val)
	}

	stats.DecTX(ptp.MessageSync)
	if val := stats.tx.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected tx to be 0, got %d", val)
	}
}

func TestJSONStatsRXSignaling(t *testing.T) {
	stats := NewJSONStats()

	stats.IncRXSignalingGrant(ptp.MessageSync)
	stats.IncRXSignalingCancel(ptp.MessageSync)
	if val := stats.rxSignalingGrant.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected rxSignalingGrant to be 1, got %d", val)
	}
	if val := stats.rxSignalingCancel.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected rxSignalingCancel to be 1, got %d", val)
	}

	stats.DecRXSignalingGrant(ptp.MessageSync)
	stats.DecRXSignalingCancel(ptp.MessageSync)
	if val := stats.rxSignalingGrant.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected rxSignalingGrant to be 0, got %d", val)
	}
	if val := stats.rxSignalingCancel.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected rxSignalingCancel to be 0, got %d", val)
	}
}

func TestJSONStatsTXSignaling(t *testing.T) {
	stats := NewJSONStats()

	stats.IncTXSignalingGrant(ptp.MessageSync)
	stats.IncTXSignalingCancel(ptp.MessageSync)
	if val := stats.txSignalingGrant.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected txSignalingGrant to be 1, got %d", val)
	}
	if val := stats.txSignalingCancel.load(int(ptp.MessageSync)); val != 1 {
		t.Errorf("expected txSignalingCancel to be 1, got %d", val)
	}

	stats.DecTXSignalingGrant(ptp.MessageSync)
	stats.DecTXSignalingCancel(ptp.MessageSync)
	if val := stats.txSignalingGrant.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected txSignalingGrant to be 0, got %d", val)
	}
	if val := stats.txSignalingCancel.load(int(ptp.MessageSync)); val != 0 {
		t.Errorf("expected txSignalingCancel to be 0, got %d", val)
	}
}

func TestJSONStatsSetMaxWorkerQueue(t *testing.T) {
	stats := NewJSONStats()

	stats.SetMaxWorkerQueue(10, 42)
	if val := stats.workerQueue.load(10); val != 42 {
		t.Errorf("expected workerQueue to be 42, got %d", val)
	}
}

func TestJSONStatsWorkerSubs(t *testing.T) {
	stats := NewJSONStats()

	stats.IncWorkerSubs(10)
	if val := stats.workerSubs.load(10); val != 1 {
		t.Errorf("expected workerSubs to be 1, got %d", val)
	}

	stats.DecWorkerSubs(10)
	// Original test had a bug, it checked stats.tx instead of stats.workerSubs
	if val := stats.workerSubs.load(10); val != 0 {
		t.Errorf("expected workerSubs to be 0, got %d", val)
	}
}

func TestJSONStatsSetMaxTXTSAttempts(t *testing.T) {
	stats := NewJSONStats()

	stats.SetMaxTXTSAttempts(10, 42)
	if val := stats.txtsattempts.load(10); val != 42 {
		t.Errorf("expected txtsattempts to be 42, got %d", val)
	}
}

func TestJSONStatsSetUTCOffset(t *testing.T) {
	stats := NewJSONStats()

	stats.SetUTCOffsetSec(42)
	if stats.utcoffsetSec != 42 {
		t.Errorf("expected utcoffsetSec to be 42, got %d", stats.utcoffsetSec)
	}
}

func TestJSONStatsSetClockAccuracy(t *testing.T) {
	stats := NewJSONStats()

	stats.SetClockAccuracy(42)
	if stats.clockaccuracy != 42 {
		t.Errorf("expected clockaccuracy to be 42, got %d", stats.clockaccuracy)
	}
}

func TestJSONStatsSetClockCLass(t *testing.T) {
	stats := NewJSONStats()

	stats.SetClockClass(42)
	if stats.clockclass != 42 {
		t.Errorf("expected clockclass to be 42, got %d", stats.clockclass)
	}
}

func TestJSONStatsSetDrain(t *testing.T) {
	stats := NewJSONStats()

	stats.SetDrain(1)
	if stats.drain != 1 {
		t.Errorf("expected drain to be 1, got %d", stats.drain)
	}
}

func TestJSONStatsSnapshot(t *testing.T) {
	stats := NewJSONStats()

	go stats.Start(0)
	time.Sleep(time.Millisecond)

	stats.IncSubscription(ptp.MessageAnnounce)
	stats.IncTX(ptp.MessageSync)
	stats.IncTX(ptp.MessageSync)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.SetClockAccuracy(1)
	stats.SetClockClass(1)
	stats.SetUTCOffsetSec(1)
	stats.SetDrain(1)
	stats.IncReload()
	stats.IncTXTSMissing()

	stats.Snapshot()

	expectedStats := counters{}
	expectedStats.init()
	expectedStats.subscriptions.store(int(ptp.MessageAnnounce), 1)
	expectedStats.tx.store(int(ptp.MessageSync), 2)
	expectedStats.rxSignalingGrant.store(int(ptp.MessageDelayResp), 3)
	expectedStats.utcoffsetSec = 1
	expectedStats.clockaccuracy = 1
	expectedStats.clockclass = 1
	expectedStats.drain = 1
	expectedStats.reload = 1
	expectedStats.txtsMissing = 1

	if !reflect.DeepEqual(expectedStats.subscriptions.m, stats.report.subscriptions.m) {
		t.Errorf("subscriptions map mismatch: \n got: %v\nwant: %v", stats.report.subscriptions.m, expectedStats.subscriptions.m)
	}
	if !reflect.DeepEqual(expectedStats.tx.m, stats.report.tx.m) {
		t.Errorf("tx map mismatch: \n got: %v\nwant: %v", stats.report.tx.m, expectedStats.tx.m)
	}
	if !reflect.DeepEqual(expectedStats.rxSignalingGrant.m, stats.report.rxSignalingGrant.m) {
		t.Errorf("rxSignalingGrant map mismatch: \n got: %v\nwant: %v", stats.report.rxSignalingGrant.m, expectedStats.rxSignalingGrant.m)
	}
	if expectedStats.utcoffsetSec != stats.report.utcoffsetSec {
		t.Errorf("utcoffsetSec mismatch: got %d, want %d", stats.report.utcoffsetSec, expectedStats.utcoffsetSec)
	}
	if expectedStats.clockaccuracy != stats.report.clockaccuracy {
		t.Errorf("clockaccuracy mismatch: got %d, want %d", stats.report.clockaccuracy, expectedStats.clockaccuracy)
	}
	if expectedStats.clockclass != stats.report.clockclass {
		t.Errorf("clockclass mismatch: got %d, want %d", stats.report.clockclass, expectedStats.clockclass)
	}
	if expectedStats.drain != stats.report.drain {
		t.Errorf("drain mismatch: got %d, want %d", stats.report.drain, expectedStats.drain)
	}
	if expectedStats.reload != stats.report.reload {
		t.Errorf("reload mismatch: got %d, want %d", stats.report.reload, expectedStats.reload)
	}
	if expectedStats.txtsMissing != stats.report.txtsMissing {
		t.Errorf("txtsMissing mismatch: got %d, want %d", stats.report.txtsMissing, expectedStats.txtsMissing)
	}
}

func TestJSONExport(t *testing.T) {
	stats := NewJSONStats()
	port, err := getFreePort(t)
	if err != nil {
		t.Fatalf("Failed to allocate port: %v", err)
	}
	go stats.Start(port)
	time.Sleep(time.Second)

	stats.IncSubscription(ptp.MessageAnnounce)
	stats.IncTX(ptp.MessageSync)
	stats.IncTX(ptp.MessageSync)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.IncRXSignalingGrant(ptp.MessageDelayResp)
	stats.IncRXSignalingCancel(ptp.MessageSync)
	stats.IncRXSignalingCancel(ptp.MessageSync)
	stats.SetUTCOffsetSec(1)
	stats.SetClockAccuracy(1)
	stats.SetClockClass(1)
	stats.SetDrain(1)
	stats.IncReload()
	stats.IncTXTSMissing()

	stats.Snapshot()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	defer checkClose(t, resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll failed: %v", err)
	}

	var data map[string]int64
	err = json.Unmarshal(body, &data)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedMap := make(map[string]int64)
	expectedMap["subscriptions.announce"] = 1
	expectedMap["tx.sync"] = 2
	expectedMap["rx.signaling.grant.delay_resp"] = 3
	expectedMap["rx.signaling.cancel.sync"] = 2
	expectedMap["utcoffset_sec"] = 1
	expectedMap["clockaccuracy"] = 1
	expectedMap["clockclass"] = 1
	expectedMap["drain"] = 1
	expectedMap["reload"] = 1
	expectedMap["txts.missing"] = 1

	if !reflect.DeepEqual(expectedMap, data) {
		t.Errorf("map mismatch:\n got: %v\nwant: %v", data, expectedMap)
	}
}
