package server

import (
	"fmt"
	"github.com/nats-io/gnatsd/server/health"
	"github.com/nats-io/gnatsd/server/lcon"
	"time"
)

// CreateInternalHealthClient makes an internal
// entirely in-process client that monitors
// cluster health and manages group
// membership functions like leader election.
//
func (s *Server) CreateInternalHealthClient(
	clientListenReady chan struct{},

) {

	s.mu.Lock()

	// To keep the health client fast and its traffic
	// internal-only, we use an bi-directional,
	// in-memory version of a TCP stream.
	cli, srv := lcon.NewBidir(s.info.MaxPayload * 2)

	host := s.info.Host
	port := s.info.Port
	rank := s.info.ServerRank
	beat := s.opts.PingInterval

	cfg := &health.MembershipCfg{
		MaxClockSkew: time.Second,
		BeatDur:      beat,
		MyRank:       rank,
		CliConn:      cli,
		SrvConn:      srv,
	}
	mship := health.NewMembership(cfg)
	s.healthClient = mship

	s.mu.Unlock()

	go func() {
		select {
		case <-clientListenReady:
			err := mship.Start()
			if err != nil {
				Errorf("error starting health monitor: %s", err)
			}
		case <-s.rcQuit:
		}
	}()
}
