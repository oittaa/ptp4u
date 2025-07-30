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
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/oittaa/ptp4u/dscp"
	ptp "github.com/oittaa/ptp4u/protocol"
	"github.com/oittaa/ptp4u/stats"
	"github.com/oittaa/ptp4u/timestamp"

	"golang.org/x/sys/unix"
)

// sendWorker monitors the queue of jobs
type sendWorker struct {
	mux            sync.Mutex
	id             int
	queue          chan *SubscriptionClient
	signalingQueue chan *SubscriptionClient
	config         *Config
	stats          stats.Stats

	clients map[ptp.MessageType]map[ptp.PortIdentity]*SubscriptionClient
}

func newSendWorker(i int, c *Config, st stats.Stats) *sendWorker {
	s := &sendWorker{
		id:     i,
		config: c,
		stats:  st,
	}
	s.clients = make(map[ptp.MessageType]map[ptp.PortIdentity]*SubscriptionClient)
	s.queue = make(chan *SubscriptionClient, c.QueueSize)
	s.signalingQueue = make(chan *SubscriptionClient, c.QueueSize)
	return s
}

func (s *sendWorker) listen() (eventFD, generalFD int, err error) {
	// socket domain differs depending whether we are listening on ipv4 or ipv6
	domain := unix.AF_INET6
	if s.config.IP.To4() != nil {
		domain = unix.AF_INET
	}
	// set up event connection
	eventFD, err = unix.Socket(domain, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return -1, -1, fmt.Errorf("creating event socket error: %w", err)
	}
	sockAddrAnyPort := timestamp.IPToSockaddr(s.config.IP, 0)

	// set SO_REUSEPORT so we can potentially trace network path from same source port.
	// needs to be set before we bind to a port.
	if err = unix.SetsockoptInt(eventFD, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		return -1, -1, fmt.Errorf("failed to set SO_REUSEPORT on event socket: %w", err)
	}
	// bind to any ephemeral port
	if err = unix.Bind(eventFD, sockAddrAnyPort); err != nil {
		return -1, -1, fmt.Errorf("unable to bind event socket connection: %w", err)
	}

	// get local port we'll send packets from
	localSockAddr, err := unix.Getsockname(eventFD)
	if err != nil {
		return -1, -1, fmt.Errorf("unable to find local ip: %w", err)
	}
	switch v := localSockAddr.(type) {
	case *unix.SockaddrInet4:
		slog.Info("Started worker event",
			slog.Int("worker_id", s.id),
			slog.String("ip", net.IP(v.Addr[:]).String()),
			slog.Int("port", v.Port),
		)
	case *unix.SockaddrInet6:
		slog.Info("Started worker event",
			slog.Int("worker_id", s.id),
			slog.String("ip", net.IP(v.Addr[:]).String()),
			slog.Int("port", v.Port),
		)
	default:
		slog.Error("Unexpected local addr type", slog.String("type", fmt.Sprintf("%T", v)))
	}

	if err = dscp.Enable(eventFD, s.config.IP, s.config.DSCP); err != nil {
		return -1, -1, fmt.Errorf("setting DSCP on event socket: %w", err)
	}

	// Syncs sent from event port, so need to turn on timestamping here
	if err := timestamp.EnableTimestamps(s.config.TimestampType, eventFD, s.config.Interface); err != nil {
		return -1, -1, err
	}

	// set up general connection
	generalFD, err = unix.Socket(domain, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return -1, -1, fmt.Errorf("creating general socket error: %w", err)
	}
	// set SO_REUSEPORT so we can potentially trace network path from same source port.
	// needs to be set before we bind to a port.
	if err = unix.SetsockoptInt(generalFD, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		return -1, -1, fmt.Errorf("failed to set SO_REUSEPORT on general socket: %w", err)
	}
	// bind to any ephemeral port
	if err = unix.Bind(generalFD, sockAddrAnyPort); err != nil {
		return -1, -1, fmt.Errorf("binding event socket connection: %w", err)
	}
	// enable DSCP
	if err = dscp.Enable(generalFD, s.config.IP, s.config.DSCP); err != nil {
		return -1, -1, fmt.Errorf("setting DSCP on general socket: %w", err)
	}
	return
}

// Start a SendWorker which will pull data from the queue and send Sync and Followup packets
func (s *sendWorker) Start() {
	eFd, gFd, err := s.listen()
	if err != nil {
		slog.Error("failed to start sendWorker", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := unix.Close(eFd); err != nil {
			slog.Error("failed to close file descriptor", "fd", eFd, "error", err)
		}
	}()
	defer func() {
		if err := unix.Close(gFd); err != nil {
			slog.Error("failed to close file descriptor", "fd", gFd, "error", err)
		}
	}()

	// reusable buffers
	buf := make([]byte, timestamp.PayloadSizeBytes)
	oob := make([]byte, timestamp.ControlSizeBytes)

	// TMP buffers
	toob := make([]byte, timestamp.ControlSizeBytes)
	soob := make([]byte, unix.CmsgSpace(timestamp.SizeofSeqID))

	olderKernel := false // kernels prior to 6.13 do not support SCM_TS_OPT_ID

	var (
		n        int
		attempts int
		txTS     time.Time
		c        *SubscriptionClient
	)

	for {
		select {
		case c = <-s.queue:
			switch c.subscriptionType {
			case ptp.MessageSync:
				// send sync
				c.UpdateSync()
				n, err = ptp.BytesTo(c.Sync(), buf)
				if err != nil {
					slog.Error("Failed to generate the sync packet", "error", err)
					continue
				}
				if olderKernel {
					slog.Debug("Sending sync")
					err = unix.Sendto(eFd, buf[:n], 0, c.eclisa)
					if err != nil {
						slog.Error("Failed to send the sync packet", "error", err)
						continue
					}
					s.stats.IncTX(c.subscriptionType)
					txTS, attempts, err = timestamp.ReadTXtimestampBuf(eFd, oob, toob)
					s.stats.SetMaxTXTSAttempts(s.id, int64(attempts))
					if err != nil {
						slog.Warn("Failed to read TX timestamp", "error", err)
						s.stats.IncTXTSMissing()
						continue
					}
					if s.config.TimestampType != timestamp.HW {
						txTS = txTS.Add(s.config.UTCOffset)
					}
				} else {
					seqID := uint32(c.Sync().SequenceID)
					slog.Debug("Sending sync (SCM_TS_OPT_ID set)")
					timestamp.SeqIDSocketControlMessage(seqID, soob)
					err = unix.Sendmsg(eFd, buf[:n], soob, c.eclisa, 0)
					if errors.Is(err, unix.EINVAL) {
						// EINVAL means the kernel does not support SCM_TS_OPT_ID
						// fallback to previous approach
						olderKernel = true
						slog.Warn("Failed to set SCM_TS_OPT_ID in Socket Control Message", "error", err)
						continue
					} else if err != nil {
						slog.Error("Failed to send sync packet", "error", err)
						continue
					}
					s.stats.IncTX(c.subscriptionType)
					txTS, attempts, err = timestamp.ReadTimeStampSeqIDBuf(eFd, toob, seqID)
					s.stats.SetMaxTXTSAttempts(s.id, int64(attempts))
					if err != nil {
						slog.Warn("Failed to read TX timestamp", "error", err)
						s.stats.IncTXTSMissing()
						continue
					}
				}

				// send followup
				c.UpdateFollowup(txTS)
				n, err = ptp.BytesTo(c.Followup(), buf)
				if err != nil {
					slog.Error("Failed to generate the followup packet", "error", err)
					continue
				}
				slog.Debug("Sending followup")

				err = unix.Sendto(gFd, buf[:n], 0, c.gclisa)
				if err != nil {
					slog.Error("Failed to send the followup packet", "error", err)
					continue
				}
				s.stats.IncTX(ptp.MessageFollowUp)
			case ptp.MessageAnnounce:
				// send announce
				c.UpdateAnnounce()
				n, err = ptp.BytesTo(c.Announce(), buf)
				if err != nil {
					slog.Error("Failed to prepare the announce packet", "error", err)
					continue
				}
				slog.Debug("Sending announce")

				err = unix.Sendto(gFd, buf[:n], 0, c.gclisa)
				if err != nil {
					slog.Error("Failed to send the announce packet", "error", err)
					continue
				}
				s.stats.IncTX(c.subscriptionType)
			case ptp.MessageDelayResp:
				// send delay response
				n, err = ptp.BytesTo(c.DelayResp(), buf)
				if err != nil {
					slog.Error("Failed to prepare the delay response packet", "error", err)
					continue
				}
				slog.Debug("Sending delay response")

				err = unix.Sendto(gFd, buf[:n], 0, c.gclisa)
				if err != nil {
					slog.Error("Failed to send the delay response", "error", err)
					continue
				}
				s.stats.IncTX(c.subscriptionType)
			case ptp.MessageDelayReq:
				// send sync
				n, err = ptp.BytesTo(c.Sync(), buf)
				if err != nil {
					slog.Error("Failed to generate the sync packet", "error", err)
					continue
				}

				if olderKernel {
					slog.Debug("Sending sync")
					err = unix.Sendto(eFd, buf[:n], 0, c.eclisa)
					if err != nil {
						slog.Error("Failed to send the sync packet", "error", err)
						continue
					}
					s.stats.IncTX(ptp.MessageSync)
					txTS, attempts, err = timestamp.ReadTXtimestampBuf(eFd, oob, toob)
					s.stats.SetMaxTXTSAttempts(s.id, int64(attempts))
					if err != nil {
						slog.Warn("Failed to read TX timestamp", "error", err)
						s.stats.IncTXTSMissing()
						continue
					}
					if s.config.TimestampType != timestamp.HW {
						txTS = txTS.Add(s.config.UTCOffset)
					}
				} else {
					seqID := uint32(c.Sync().SequenceID)
					slog.Debug("Sending sync (SCM_TS_OPT_ID set)")
					timestamp.SeqIDSocketControlMessage(seqID, soob)
					err = unix.Sendmsg(eFd, buf[:n], soob, c.eclisa, 0)
					if errors.Is(err, unix.EINVAL) {
						// EINVAL means the kernel does not support SCM_TS_OPT_ID
						// fallback to previous approach
						olderKernel = true
						slog.Warn("Failed to set SCM_TS_OPT_ID in Socket Control Message", "error", err)
						continue
					} else if err != nil {
						slog.Error("Failed to send sync packet", "error", err)
						continue
					}
					s.stats.IncTX(ptp.MessageSync)
					txTS, attempts, err = timestamp.ReadTimeStampSeqIDBuf(eFd, toob, seqID)
					s.stats.SetMaxTXTSAttempts(s.id, int64(attempts))
					if err != nil {
						slog.Warn("Failed to read TX timestamp", "error", err)
						s.stats.IncTXTSMissing()
						continue
					}
				}
				// send announce
				c.UpdateAnnounceFollowUp(txTS)
				n, err = ptp.BytesTo(c.Announce(), buf)
				if err != nil {
					slog.Error("Failed to prepare the announce packet", "error", err)
					continue
				}
				slog.Debug("Sending announce")

				err = unix.Sendto(gFd, buf[:n], 0, c.gclisa)
				if err != nil {
					slog.Error("Failed to send the announce packet", "error", err)
					continue
				}
				s.stats.IncTX(ptp.MessageAnnounce)
			default:
				slog.Error("Unknown subscription type", "subscriptionType", c.subscriptionType)
				continue
			}
			c.IncSequenceID()
			s.stats.SetMaxWorkerQueue(s.id, int64(len(s.queue)))
		case c = <-s.signalingQueue:
			n, err = ptp.BytesTo(c.Signaling(), buf)
			if err != nil {
				slog.Error("Failed to prepare the unicast signaling", "error", err)
				continue
			}
			err = unix.Sendto(gFd, buf[:n], 0, c.gclisa)
			if err != nil {
				slog.Error("Failed to send the unicast signaling", "error", err)
				continue
			}
			slog.Debug("Sent unicast signaling")
			for _, tlv := range c.Signaling().TLVs {
				switch tlv.(type) {
				case *ptp.GrantUnicastTransmissionTLV:
					s.stats.IncTXSignalingGrant(c.subscriptionType)
				case *ptp.CancelUnicastTransmissionTLV:
					s.stats.IncTXSignalingCancel(c.subscriptionType)
				}
			}
		}
	}
}

// FindSubscription retrieves an existing client
func (s *sendWorker) FindSubscription(clientID ptp.PortIdentity, st ptp.MessageType) *SubscriptionClient {
	s.mux.Lock()
	defer s.mux.Unlock()
	m, ok := s.clients[st]
	if !ok {
		return nil
	}
	sub, ok := m[clientID]
	if !ok {
		return nil
	}
	return sub
}

// FindClients retrieves all clients for a particular subscription type
func (s *sendWorker) FindClients(st ptp.MessageType) map[ptp.PortIdentity]*SubscriptionClient {
	s.mux.Lock()
	defer s.mux.Unlock()
	m, ok := s.clients[st]
	if !ok {
		return nil
	}
	return m
}

// RegisterSubscription will overwrite an existing subscription.
// Make sure you call findSubscription before this
func (s *sendWorker) RegisterSubscription(clientID ptp.PortIdentity, st ptp.MessageType, sc *SubscriptionClient) {
	s.mux.Lock()
	defer s.mux.Unlock()
	m, ok := s.clients[st]
	if !ok {
		s.clients[st] = map[ptp.PortIdentity]*SubscriptionClient{}
		m = s.clients[st]
	}
	m[clientID] = sc
}

func (s *sendWorker) inventoryClients() {
	s.mux.Lock()
	defer s.mux.Unlock()
	for st, subs := range s.clients {
		for k, sc := range subs {
			if !sc.Running() {
				delete(subs, k)
				continue
			}
			s.stats.IncSubscription(st)
			s.stats.IncWorkerSubs(s.id)
		}
	}
}
