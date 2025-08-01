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

package dscp

import (
	"io"
	"net"
	"testing"

	"github.com/oittaa/ptp4u/timestamp"
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

func TestEnableDSCP(t *testing.T) {
	conn4, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen on udp4: %v", err)
	}
	defer checkClose(t, conn4)
	// get connection file descriptor
	fd4, err := timestamp.ConnFd(conn4)
	if err != nil {
		t.Fatalf("failed to get conn fd4: %v", err)
	}
	err = Enable(fd4, net.ParseIP("127.0.0.1"), 42)
	if err != nil {
		t.Fatalf("failed to enable dscp on fd4: %v", err)
	}

	conn6, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("::"), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen on udp6: %v", err)
	}
	defer checkClose(t, conn6)
	// get connection file descriptor
	fd6, err := timestamp.ConnFd(conn6)
	if err != nil {
		t.Fatalf("failed to get conn fd6: %v", err)
	}
	err = Enable(fd6, net.ParseIP("::"), 42)
	if err != nil {
		t.Fatalf("failed to enable dscp on fd6: %v", err)
	}
}
