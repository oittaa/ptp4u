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

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/oittaa/ptp4u/drain"
	"github.com/oittaa/ptp4u/server"
	"github.com/oittaa/ptp4u/stats"
	"github.com/oittaa/ptp4u/timestamp"
)

func main() {
	c := &server.Config{
		DynamicConfig: *server.NewDefaultDynamicConfig(),
	}

	var ipaddr string

	flag.IntVar(&c.DSCP, "dscp", 0, "DSCP for PTP packets, valid values are between 0-63 (used by send workers)")
	flag.IntVar(&c.MonitoringPort, "monitoringport", 8888, "Port to run monitoring server on")
	flag.IntVar(&c.QueueSize, "queue", 0, "Size of the queue to send out packets")
	flag.IntVar(&c.RecvWorkers, "recvworkers", 10, "Set the number of receive workers")
	flag.IntVar(&c.SendWorkers, "workers", 100, "Set the number of send workers")
	flag.UintVar(&c.DomainNumber, "domainnumber", 0, "Set the PTP domain by its number. Valid values are [0-255]")
	flag.StringVar(&c.ConfigFile, "config", "", "Path to a config with dynamic settings")
	flag.StringVar(&c.DebugAddr, "pprofaddr", "", "host:port for the pprof to bind")
	flag.StringVar(&c.Interface, "iface", "eth0", "Set the interface")
	flag.StringVar(&c.LogLevel, "loglevel", "warn", "Set a log level. Can be: debug, info, warn, error")
	flag.StringVar(&c.PidFile, "pidfile", "/var/run/ptp4u.pid", "Pid file location")
	flag.TextVar(&c.TimestampType, "timestamptype", timestamp.HW, fmt.Sprintf("Timestamp type. Can be: %s, %s", timestamp.HW, timestamp.SW))
	flag.StringVar(&ipaddr, "ip", "::", "IP to bind on")
	flag.StringVar(&c.DrainFileName, "drainfile", "/var/tmp/kill_ptp4u", "ptp4u drain file location")
	flag.StringVar(&c.UndrainFileName, "undrainfile", "/var/tmp/unkill_ptp4u", "ptp4u force undrain file location")
	flag.Parse()

	var level slog.Level
	switch c.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warning":
		fallthrough
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		slog.Error("Unrecognized log level.", "loglevel", c.LogLevel)
		os.Exit(1)
	}
	_ = slog.SetLogLoggerLevel(level)

	if c.ConfigFile != "" {
		dc, err := server.ReadDynamicConfig(c.ConfigFile)
		if err != nil {
			slog.Error("Can't read the config file.", "error", err)
			os.Exit(1)
		}
		c.DynamicConfig = *dc
	}

	if c.DSCP < 0 || c.DSCP > 63 {
		slog.Error("Unsupported DSCP value.", "dscp", c.DSCP)
		os.Exit(1)
	}

	if c.DomainNumber > 255 {
		slog.Error("Unsupported DomainNumber value.", "domainnumber", c.DomainNumber)
		os.Exit(1)
	}

	if c.RecvWorkers <= 0 {
		slog.Error("Number of receive workers must be greater than zero.", "recvworkers", c.RecvWorkers)
		os.Exit(1)
	}

	if c.SendWorkers <= 0 {
		slog.Error("Number of send workers must be greater than zero.", "workers", c.SendWorkers)
		os.Exit(1)
	}

	if c.MonitoringPort <= 0 || c.MonitoringPort > 65535 {
		slog.Error("Invalid monitoring port.", "port", c.MonitoringPort)
		os.Exit(1)
	}

	if c.QueueSize < 0 {
		slog.Error("Queue size cannot be negative.", "queue", c.QueueSize)
		os.Exit(1)
	}

	switch c.TimestampType {
	case timestamp.SW:
		slog.Warn("Software timestamps greatly reduce the precision")
		fallthrough
	case timestamp.HW:
		slog.Debug("Using:", "timestamptype", c.TimestampType)
	default:
		slog.Error("Unrecognized:", "timestamptype", c.TimestampType)
		os.Exit(1)
	}

	c.IP = net.ParseIP(ipaddr)
	if c.IP == nil {
		slog.Error("Invalid IP address provided.", "ip", ipaddr)
		os.Exit(1)
	}
	found, err := c.IfaceHasIP()
	if err != nil {
		slog.Error("Checking IP failed.", "iface", c.Interface, "error", err)
		os.Exit(1)
	}
	if !found {
		slog.Error("IP not found on interface.", "ip", c.IP, "iface", c.Interface)
		os.Exit(1)
	}

	if c.DebugAddr != "" {
		if _, _, err := net.SplitHostPort(c.DebugAddr); err != nil {
			slog.Error("Invalid pprof address format. Expected host:port", "pprofaddr", c.DebugAddr, "error", err)
			os.Exit(1)
		}
		slog.Warn("Starting profiler.", "pprofaddr", c.DebugAddr)
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		pprofServer := &http.Server{
			Addr:         c.DebugAddr,
			Handler:      pprofMux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		}
		go func() {
			err := pprofServer.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				slog.Error("pprof server failed to start or crashed.", "pprofaddr", c.DebugAddr, "error", err)
				os.Exit(1)
			}
			slog.Info("pprof server stopped.", "pprofaddr", c.DebugAddr)
		}()
	}

	slog.Info("UTC offset:", "UTCOffset", c.UTCOffset)

	// Monitoring
	// Replace with your implementation of Stats
	st := stats.NewJSONStats()
	go st.Start(c.MonitoringPort)

	// drain check
	check := &drain.FileDrain{FileName: c.DrainFileName}
	checks := []drain.Drain{check}

	s := server.Server{
		Config: c,
		Stats:  st,
		Checks: checks,
	}

	if err := s.Start(); err != nil {
		slog.Error("Server run failed:", "error", err)
		os.Exit(1)
	}
}
