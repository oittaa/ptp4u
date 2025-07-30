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
	"reflect"
	"testing"

	ptp "github.com/oittaa/ptp4u/protocol"
)

func TestSyncMapInt64Keys(t *testing.T) {
	s := syncMapInt64{}
	s.init()

	expected := []int{24, 42}
	for _, i := range expected {
		s.inc(i)
	}

	found := 0
	for _, k := range s.keys() {
		for _, i := range expected {
			if i == k {
				found++
				break
			}
		}
	}

	if len(expected) != found {
		t.Errorf("expected to find %d keys, but found %d", len(expected), found)
	}
}

func TestSyncMapInt64Copy(t *testing.T) {
	s := syncMapInt64{}
	s.init()

	s.store(1, 1)
	if val := s.load(1); val != 1 {
		t.Errorf("expected s.load(1) to be 1, got %d", val)
	}

	dst := syncMapInt64{}
	dst.init()

	s.copy(&dst)
	if !reflect.DeepEqual(s.m, dst.m) {
		t.Errorf("expected maps to be equal. got=%v, want=%v", dst.m, s.m)
	}
	if val := dst.load(1); val != 1 {
		t.Errorf("expected dst.load(1) to be 1, got %d", val)
	}
}

func TestSyncMapInt64Counters(t *testing.T) {
	c := counters{}
	c.init()

	c.subscriptions.store(1, 1)
	c.rx.store(1, 1)
	c.tx.store(1, 1)
	c.rxSignalingGrant.store(1, 1)
	c.txSignalingCancel.store(1, 1)
	c.workerQueue.store(1, 1)
	c.workerSubs.store(1, 1)
	c.txtsattempts.store(1, 1)
	c.utcoffsetSec = 1
	c.clockaccuracy = 1
	c.clockclass = 1
	c.drain = 1
	c.reload = 1
	c.txtsMissing = 5

	if val := c.subscriptions.load(1); val != 1 {
		t.Errorf("expected c.subscriptions.load(1) to be 1, got %d", val)
	}
	if val := c.rx.load(1); val != 1 {
		t.Errorf("expected c.rx.load(1) to be 1, got %d", val)
	}
	if val := c.tx.load(1); val != 1 {
		t.Errorf("expected c.tx.load(1) to be 1, got %d", val)
	}
	if val := c.rxSignalingGrant.load(1); val != 1 {
		t.Errorf("expected c.rxSignalingGrant.load(1) to be 1, got %d", val)
	}
	if val := c.txSignalingCancel.load(1); val != 1 {
		t.Errorf("expected c.txSignalingCancel.load(1) to be 1, got %d", val)
	}
	if val := c.workerQueue.load(1); val != 1 {
		t.Errorf("expected c.workerQueue.load(1) to be 1, got %d", val)
	}
	if val := c.workerSubs.load(1); val != 1 {
		t.Errorf("expected c.workerSubs.load(1) to be 1, got %d", val)
	}
	if val := c.txtsattempts.load(1); val != 1 {
		t.Errorf("expected c.txtsattempts.load(1) to be 1, got %d", val)
	}
	if c.utcoffsetSec != 1 {
		t.Errorf("expected c.utcoffsetSec to be 1, got %d", c.utcoffsetSec)
	}
	if c.clockaccuracy != 1 {
		t.Errorf("expected c.clockaccuracy to be 1, got %d", c.clockaccuracy)
	}
	if c.clockclass != 1 {
		t.Errorf("expected c.clockclass to be 1, got %d", c.clockclass)
	}
	if c.drain != 1 {
		t.Errorf("expected c.drain to be 1, got %d", c.drain)
	}
	if c.reload != 1 {
		t.Errorf("expected c.reload to be 1, got %d", c.reload)
	}
	if c.txtsMissing != 5 {
		t.Errorf("expected c.txtsMissing to be 5, got %d", c.txtsMissing)
	}

	c.reset()

	if val := c.subscriptions.load(1); val != 0 {
		t.Errorf("expected c.subscriptions.load(1) to be 0, got %d", val)
	}
	if val := c.rx.load(1); val != 0 {
		t.Errorf("expected c.rx.load(1) to be 0, got %d", val)
	}
	if val := c.tx.load(1); val != 0 {
		t.Errorf("expected c.tx.load(1) to be 0, got %d", val)
	}
	if val := c.rxSignalingGrant.load(1); val != 0 {
		t.Errorf("expected c.rxSignalingGrant.load(1) to be 0, got %d", val)
	}
	if val := c.txSignalingCancel.load(1); val != 0 {
		t.Errorf("expected c.txSignalingCancel.load(1) to be 0, got %d", val)
	}
	if val := c.workerQueue.load(1); val != 0 {
		t.Errorf("expected c.workerQueue.load(1) to be 0, got %d", val)
	}
	if val := c.workerSubs.load(1); val != 0 {
		t.Errorf("expected c.workerSubs.load(1) to be 0, got %d", val)
	}
	if val := c.txtsattempts.load(1); val != 0 {
		t.Errorf("expected c.txtsattempts.load(1) to be 0, got %d", val)
	}
	if c.utcoffsetSec != 0 {
		t.Errorf("expected c.utcoffsetSec to be 0, got %d", c.utcoffsetSec)
	}
	if c.clockaccuracy != 0 {
		t.Errorf("expected c.clockaccuracy to be 0, got %d", c.clockaccuracy)
	}
	if c.clockclass != 0 {
		t.Errorf("expected c.clockclass to be 0, got %d", c.clockclass)
	}
	if c.drain != 0 {
		t.Errorf("expected c.drain to be 0, got %d", c.drain)
	}
	if c.reload != 0 {
		t.Errorf("expected c.reload to be 0, got %d", c.reload)
	}
	if c.txtsMissing != 0 {
		t.Errorf("expected c.txtsMissing to be 0, got %d", c.txtsMissing)
	}
}

func TestCountersToMap(t *testing.T) {
	c := counters{}
	c.init()

	c.subscriptions.store(int(ptp.MessageAnnounce), 1)
	c.tx.store(int(ptp.MessageSync), 2)
	c.rxSignalingGrant.store(int(ptp.MessageDelayResp), 3)
	c.rxSignalingCancel.store(int(ptp.MessageSync), 1)
	c.utcoffsetSec = 1
	c.clockaccuracy = 42
	c.clockclass = 6
	c.drain = 1
	c.reload = 2
	c.txtsMissing = 5

	result := c.toMap()

	expectedMap := make(map[string]int64)
	expectedMap["subscriptions.announce"] = 1
	expectedMap["tx.sync"] = 2
	expectedMap["rx.signaling.grant.delay_resp"] = 3
	expectedMap["rx.signaling.cancel.sync"] = 1
	expectedMap["utcoffset_sec"] = 1
	expectedMap["clockaccuracy"] = 42
	expectedMap["clockclass"] = 6
	expectedMap["drain"] = 1
	expectedMap["reload"] = 2
	expectedMap["txts.missing"] = 5

	if !reflect.DeepEqual(expectedMap, result) {
		t.Errorf("map mismatch:\n got: %v\nwant: %v", result, expectedMap)
	}
}
