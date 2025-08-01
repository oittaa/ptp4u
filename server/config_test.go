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
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"golang.org/x/sys/unix"
)

func TestConfigifaceIPs(t *testing.T) {
	ips, err := ifaceIPs("lo")
	if err != nil {
		t.Fatalf("ifaceIPs(\"lo\") failed: %v", err)
	}

	// Note: Depending on the system, 'lo' might have more IPs (like fe80::1).
	// This test checks for the presence of essential loopback IPs.
	los := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		// In some CI environments, IPv6 might not be configured on loopback.
		los = []net.IP{net.ParseIP("127.0.0.1")}
	}

	foundMap := make(map[string]bool)
	for _, ip := range ips {
		foundMap[ip.String()] = true
	}

	for _, lo := range los {
		if !foundMap[lo.String()] {
			t.Errorf("expected to find IP %s in interface 'lo', but it was not found. Found IPs: %v", lo, ips)
		}
	}
}

func TestConfigIfaceHasIP(t *testing.T) {
	c := Config{StaticConfig: StaticConfig{Interface: "lo"}}

	// Test with an IP that should exist on the loopback interface
	c.IP = net.ParseIP("127.0.0.1")
	found, err := c.IfaceHasIP()
	if err != nil {
		t.Fatalf("IfaceHasIP() with IP %s failed: %v", c.IP, err)
	}
	if !found {
		t.Errorf("expected IfaceHasIP() to return true for IP %s on 'lo'", c.IP)
	}

	// Test with an IP that should NOT exist on the loopback interface
	c.IP = net.ParseIP("1.2.3.4")
	found, err = c.IfaceHasIP()
	if err != nil {
		t.Fatalf("IfaceHasIP() with IP %s failed: %v", c.IP, err)
	}
	if found {
		t.Errorf("expected IfaceHasIP() to return false for IP %s on 'lo'", c.IP)
	}

	// Test with a non-existent interface
	c = Config{StaticConfig: StaticConfig{Interface: "lol-does-not-exist"}}
	c.IP = net.ParseIP("127.0.0.1")
	found, err = c.IfaceHasIP()
	if err == nil {
		t.Fatal("expected an error for non-existent interface, but got nil")
	}
	if found {
		t.Error("expected IfaceHasIP() to return false for non-existent interface")
	}
}

func TestReadDynamicConfigOk(t *testing.T) {
	expected := &DynamicConfig{
		ClockAccuracy:  0x21, // 33
		ClockClass:     6,
		DrainInterval:  2 * time.Second,
		MaxSubDuration: 3 * time.Hour,
		MetricInterval: 4 * time.Minute,
		MinSubInterval: 5 * time.Second,
		UTCOffset:      37 * time.Second,
	}

	// Test with empty path
	dc, err := ReadDynamicConfig("")
	if err == nil {
		t.Fatal("expected error for empty config path, got nil")
	}
	if dc != nil {
		t.Fatalf("expected nil config for empty path, got %+v", dc)
	}

	// Create temp file for testing
	cfg, err := os.CreateTemp("", "ptp4u-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer checkRemove(t, cfg.Name())

	config := `clockaccuracy: 33
clockclass: 6
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
	checkClose(t, cfg) // Close the file to ensure content is flushed

	dc, err = ReadDynamicConfig(cfg.Name())
	if err != nil {
		t.Fatalf("ReadDynamicConfig failed: %v", err)
	}
	if !reflect.DeepEqual(expected, dc) {
		t.Errorf("config mismatch:\ngot:  %+v\nwant: %+v", dc, expected)
	}
}

func TestReadDynamicConfigValidation(t *testing.T) {
	// baseConfig returns a valid config struct to be modified by each test case.
	baseConfig := func() DynamicConfig {
		return DynamicConfig{
			ClockAccuracy:  33,
			ClockClass:     6,
			DrainInterval:  1 * time.Second,
			MaxSubDuration: 1 * time.Hour,
			MetricInterval: 1 * time.Minute,
			MinSubInterval: 1 * time.Second,
			UTCOffset:      37 * time.Second,
		}
	}

	testCases := []struct {
		name      string
		modify    func(c *DynamicConfig) // Function to make the config invalid for the test
		expectErr error
	}{
		{
			name: "invalid clock class",
			modify: func(c *DynamicConfig) {
				c.ClockClass = 255
			},
			expectErr: errInvalidClockClass,
		},
		{
			name: "invalid utc offset",
			modify: func(c *DynamicConfig) {
				c.UTCOffset = 1 * time.Second
			},
			expectErr: errInsaneUTCoffset,
		},
		{
			name: "negative duration",
			modify: func(c *DynamicConfig) {
				c.MetricInterval = -1 * time.Second
			},
			expectErr: errNegativeDuration,
		},
		{
			name: "inconsistent intervals",
			modify: func(c *DynamicConfig) {
				c.MaxSubDuration = 1 * time.Second
				c.MinSubInterval = 2 * time.Second
			},
			expectErr: errInconsistentSubInt,
		},
		{
			name: "invalid clock accuracy",
			modify: func(c *DynamicConfig) {
				c.ClockAccuracy = 1 // 0x01 is invalid
			},
			expectErr: errInvalidClockAccuracy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a valid config and then modify it to be invalid for the test.
			cfg := baseConfig()
			tc.modify(&cfg)

			// Marshal the struct to YAML, ensuring valid syntax.
			yamlData, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatalf("failed to marshal test config to yaml: %v", err)
			}

			// Write the generated YAML to a temporary file.
			tmpFile, err := os.CreateTemp("", "ptp4u-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer checkRemove(t, tmpFile.Name())

			_, err = tmpFile.Write(yamlData)
			if err != nil {
				t.Fatalf("failed to write to temp config file: %v", err)
			}
			checkClose(t, tmpFile)

			// Run ReadDynamicConfig and check for the expected validation error.
			dc, err := ReadDynamicConfig(tmpFile.Name())
			if !errors.Is(err, tc.expectErr) {
				t.Fatalf("expected error containing %q, got %q", tc.expectErr, err)
			}
			if dc != nil {
				t.Fatalf("expected nil config on error, got %+v", dc)
			}
		})
	}
}

func TestReadDynamicConfigDamaged(t *testing.T) {
	config := "Random stuff that is not yaml"
	cfg, err := os.CreateTemp("", "ptp4u-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer checkRemove(t, cfg.Name())

	_, err = cfg.WriteString(config)
	if err != nil {
		t.Fatalf("failed to write to temp config file: %v", err)
	}
	checkClose(t, cfg)

	dc, err := ReadDynamicConfig(cfg.Name())
	if err == nil {
		t.Fatal("expected error for damaged config, got nil")
	}
	if dc != nil {
		t.Fatalf("expected nil config for damaged file, got %+v", dc)
	}
}

func TestWriteDynamicConfig(t *testing.T) {
	expected := `clockaccuracy: 33
clockclass: 6
draininterval: 2s
maxsubduration: 3h0m0s
metricinterval: 4m0s
minsubinterval: 5s
utcoffset: 37s
`
	dc := &DynamicConfig{
		ClockAccuracy:  0x21, // 33
		ClockClass:     6,
		DrainInterval:  2 * time.Second,
		MaxSubDuration: 3 * time.Hour,
		MetricInterval: 4 * time.Minute,
		MinSubInterval: 5 * time.Second,
		UTCOffset:      37 * time.Second,
	}

	// Use a path in the temp directory that doesn't exist yet
	cfgPath := fmt.Sprintf("%s/ptp4u-test-write.yaml", os.TempDir())
	// Clean up just in case of a previous failed run
	_ = os.Remove(cfgPath)
	noFileExists(t, cfgPath)

	err := dc.Write(cfgPath)
	defer checkRemove(t, cfgPath)
	if err != nil {
		t.Fatalf("dc.Write() failed: %v", err)
	}

	rl, err := os.ReadFile(cfgPath) // #nosec:G304
	if err != nil {
		t.Fatalf("os.ReadFile() failed: %v", err)
	}
	if expected != string(rl) {
		t.Errorf("config content mismatch:\ngot:\n%s\nwant:\n%s", string(rl), expected)
	}
}

func TestDynamicConfigSanityCheck(t *testing.T) {
	// A baseline valid config to be modified by test cases
	baseConfig := func() *DynamicConfig {
		return &DynamicConfig{
			ClockAccuracy:  0x21, // within 100 ns
			ClockClass:     6,    // T-BC, Locked
			DrainInterval:  10 * time.Second,
			MaxSubDuration: 1 * time.Hour,
			MinSubInterval: 1 * time.Second,
			UTCOffset:      37 * time.Second,
		}
	}

	testCases := []struct {
		name      string
		config    *DynamicConfig
		expectErr error
	}{
		{
			name:      "valid config",
			config:    baseConfig(),
			expectErr: nil,
		},
		{
			name: "insane utc offset too low",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.UTCOffset = 29 * time.Second
				return c
			}(),
			expectErr: errInsaneUTCoffset,
		},
		{
			name: "negative duration",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.MetricInterval = -1 * time.Minute
				return c
			}(),
			expectErr: errNegativeDuration,
		},
		{
			name: "inconsistent subscription interval",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.MinSubInterval = 10 * time.Second
				c.MaxSubDuration = 9 * time.Second
				return c
			}(),
			expectErr: errInconsistentSubInt,
		},
		{
			name: "invalid clock class slave only",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.ClockClass = 255
				return c
			}(),
			expectErr: errInvalidClockClass,
		},
		{
			name: "valid clock class alternate profile",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.ClockClass = 130
				return c
			}(),
			expectErr: nil,
		},
		{
			name: "invalid clock accuracy",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.ClockAccuracy = 0x70 // Not in any valid range
				return c
			}(),
			expectErr: errInvalidClockAccuracy,
		},
		{
			name: "valid clock accuracy unknown",
			config: func() *DynamicConfig {
				c := baseConfig()
				c.ClockAccuracy = 0xFE
				return c
			}(),
			expectErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.SanityCheck()
			if !errors.Is(err, tc.expectErr) {
				t.Errorf("SanityCheck() error mismatch:\ngot:  %v\nwant: %v", err, tc.expectErr)
			}
		})
	}
}

func TestPidFile(t *testing.T) {
	cfgFile, err := os.CreateTemp("", "ptp4u-pid-*.pid")
	if err != nil {
		t.Fatalf("failed to create temp pid file: %v", err)
	}
	pidFileName := cfgFile.Name()
	checkClose(t, cfgFile)

	c := &Config{StaticConfig: StaticConfig{PidFile: pidFileName}}
	// #nosec:G306
	if err := os.WriteFile(pidFileName, []byte("rubbish"), 0644); err != nil {
		t.Fatalf("failed to write rubbish to pid file: %v", err)
	}
	pid, err := ReadPidFile(c.PidFile)
	if err == nil {
		t.Fatal("expected error when reading rubbish pid file, but got nil")
	}
	if pid != 0 {
		t.Errorf("expected pid to be 0 on error, got %d", pid)
	}
	checkRemove(t, pidFileName)
	noFileExists(t, pidFileName)

	err = c.CreatePidFile()
	if err != nil {
		t.Fatalf("c.CreatePidFile() failed: %v", err)
	}
	fileExists(t, c.PidFile)

	content, err := os.ReadFile(c.PidFile)
	if err != nil {
		t.Fatalf("failed to read pid file: %v", err)
	}
	writtenPid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("failed to parse pid from file: %v", err)
	}
	if unix.Getpid() != writtenPid {
		t.Errorf("pid mismatch: expected %d, got %d", unix.Getpid(), writtenPid)
	}

	pid, err = ReadPidFile(c.PidFile)
	if err != nil {
		t.Fatalf("ReadPidFile failed: %v", err)
	}
	if unix.Getpid() != pid {
		t.Errorf("pid mismatch: expected %d, got %d", unix.Getpid(), pid)
	}

	err = c.DeletePidFile()
	if err != nil {
		t.Fatalf("c.DeletePidFile() failed: %v", err)
	}
	noFileExists(t, c.PidFile)
}
