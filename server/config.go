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

/*
Package server implements simple Unicast PTP UDP server.
*/
package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ptp "github.com/oittaa/ptp4u/protocol"
	"github.com/oittaa/ptp4u/timestamp"

	"github.com/goccy/go-yaml"
	"golang.org/x/sys/unix"
)

var (
	errInsaneUTCoffset      = errors.New("UTC offset is outside of sane range")
	errNegativeDuration     = errors.New("duration values cannot be negative")
	errInconsistentSubInt   = errors.New("maxsubduration must be greater than or equal to minsubinterval")
	errInvalidClockClass    = errors.New("invalid clock class")
	errInvalidClockAccuracy = errors.New("invalid clock accuracy")
)

// dcMux is a dynamic config mutex
var dcMux = sync.Mutex{}

// StaticConfig is a set of static options which require a server restart
type StaticConfig struct {
	ConfigFile      string
	DebugAddr       string
	DomainNumber    uint
	DrainFileName   string
	DSCP            int
	Interface       string
	IP              net.IP
	LogLevel        string
	MonitoringPort  int
	PidFile         string
	QueueSize       int
	RecvWorkers     int
	SendWorkers     int
	TimestampType   timestamp.Timestamp
	UndrainFileName string
}

// DynamicConfig is a set of dynamic options which don't need a server restart
type DynamicConfig struct {
	// ClockAccuracy to report via announce messages. Time Accurate within 100ns
	ClockAccuracy ptp.ClockAccuracy `yaml:"clockaccuracy"`
	// ClockClass to report via announce messages. 6 - Locked with Primary Reference Clock
	ClockClass ptp.ClockClass `yaml:"clockclass"`
	// DrainInterval is an interval for drain checks
	DrainInterval time.Duration `yaml:"draininterval"`
	// MaxSubDuration is a maximum sync/announce/delay_resp subscription duration
	MaxSubDuration time.Duration `yaml:"maxsubduration"`
	// MetricInterval is an interval of resetting metrics
	MetricInterval time.Duration `yaml:"metricinterval"`
	// MinSubInterval is a minimum interval of the sync/announce subscription messages
	MinSubInterval time.Duration `yaml:"minsubinterval"`
	// UTCOffset is a current UTC offset.
	UTCOffset time.Duration `yaml:"utcoffset"`
}

// Config is a server config structure
type Config struct {
	StaticConfig
	DynamicConfig

	clockIdentity ptp.ClockIdentity
}

// Set reasonable defaults for DynamicConfig
func NewDefaultDynamicConfig() *DynamicConfig {
	return &DynamicConfig{
		// Default to reporting accuracy of 100ns
		ClockAccuracy: 0x21,
		// Default to a Class 6, indicating a server locked to a primary reference
		ClockClass:     6,
		DrainInterval:  30 * time.Second,
		MaxSubDuration: 1 * time.Hour,
		MetricInterval: 1 * time.Minute,
		MinSubInterval: 1 * time.Second,
		// As of 2025 TAI UTC offset is 37 seconds
		UTCOffset: 37 * time.Second,
	}
}

// SanityCheck performs a validation of the dynamic configuration.
func (dc *DynamicConfig) SanityCheck() error {
	// Checks if UTC offset value has an adequate value
	if dc.UTCOffset < 30*time.Second || dc.UTCOffset > 50*time.Second {
		return errInsaneUTCoffset
	}

	// Check for negative durations
	if dc.DrainInterval < 0 || dc.MaxSubDuration < 0 || dc.MetricInterval < 0 || dc.MinSubInterval < 0 {
		return errNegativeDuration
	}

	// Check for logical interval consistency
	if dc.MaxSubDuration < dc.MinSubInterval {
		return fmt.Errorf("%w: max (%v) is less than min (%v)", errInconsistentSubInt, dc.MaxSubDuration, dc.MinSubInterval)
	}

	// Allow everything else except a slave-only clock
	if dc.ClockClass == ptp.ClockClassSlaveOnly {
		return fmt.Errorf("%w: %d", errInvalidClockClass, dc.ClockClass)
	}

	// Validate PTP ClockAccuracy
	if dc.ClockAccuracy < ptp.ClockAccuracyNanosecond25 || dc.ClockAccuracy > ptp.ClockAccuracySecondGreater10 {
		if dc.ClockAccuracy != ptp.ClockAccuracyUnknown {
			return fmt.Errorf("%w: %#x", errInvalidClockAccuracy, dc.ClockAccuracy)
		}
	}

	return nil
}

// ReadDynamicConfig reads dynamic config from the file
func ReadDynamicConfig(path string) (*DynamicConfig, error) {
	dc := NewDefaultDynamicConfig()
	cData, err := os.ReadFile(path) // #nosec:G304
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(cData, dc)
	if err != nil {
		return nil, err
	}

	if err := dc.SanityCheck(); err != nil {
		return nil, fmt.Errorf("dynamic config validation failed: %w", err)
	}

	return dc, nil
}

// Write dynamic config to a file
func (dc *DynamicConfig) Write(path string) error {
	d, err := yaml.Marshal(dc)
	if err != nil {
		return err
	}

	return os.WriteFile(path, d, 0644) // #nosec:G306
}

// IfaceHasIP checks if selected IP is on interface
func (c *Config) IfaceHasIP() (bool, error) {
	ips, err := ifaceIPs(c.Interface)
	if err != nil {
		return false, err
	}

	for _, ip := range ips {
		if c.IP.Equal(ip) {
			return true, nil
		}
	}

	return false, nil
}

// CreatePidFile creates a pid file in a defined location
func (c *Config) CreatePidFile() error {
	return os.WriteFile(c.PidFile, []byte(fmt.Sprintf("%d\n", unix.Getpid())), 0644) // #nosec:G306
}

// DeletePidFile deletes a pid file from a defined location
func (c *Config) DeletePidFile() error {
	return os.Remove(c.PidFile)
}

// ReadPidFile read a pid file from a path location and returns a pid
func ReadPidFile(path string) (int, error) {
	content, err := os.ReadFile(path) // #nosec:G304
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.ReplaceAll(string(content), "\n", ""))
}

// ifaceIPs gets all IPs on the specified interface
func ifaceIPs(iface string) ([]net.IP, error) {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, err
	}

	addrs, err := i.Addrs()
	if err != nil {
		return nil, err
	}

	res := []net.IP{}
	for _, addr := range addrs {
		ip := addr.(*net.IPNet).IP
		res = append(res, ip)
	}
	res = append(res, net.IPv6zero)
	res = append(res, net.IPv4zero)

	return res, nil
}
