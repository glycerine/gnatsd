package server

import (
	"fmt"
	"github.com/nats-io/gnatsd/server/health"
	"time"
)

func (s *Server) CreateInternalHealthCheckClient(
	clientListenReady chan struct{},

) {

	s.mu.Lock()
	host := s.info.Host
	port := s.info.Port
	rank := s.info.ServerRank

	cfg := &health.MembershipCfg{
		MaxClockSkew: time.Second,
		BeatDur:      100 * time.Millisecond,
		NatsUrl:      fmt.Sprintf("nats://%v:%v", host, port),
		MyRank:       rank,
	}
	mship := health.NewMembership(cfg)
	s.clusterhealth = mship

	s.mu.Unlock()

	go func() {
		select {
		case <-clientListenReady:
			err := mship.Start()
			if err != nil {
				Errorf("error starting Membership health monitor: %s", err)
			}
		case <-s.rcQuit:
		}
	}()
}
