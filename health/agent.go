package health

import (
	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/gnatsd/server/lcon"
	"net"
	"time"
)

type Agent struct {
	opts  *server.Options
	mship *Membership
}

func NewAgent(opts *server.Options) *Agent {
	return &Agent{
		opts: opts,
	}
}

func (h *Agent) Name() string {
	return "health-agent"
}

// Start makes an internal
// entirely in-process client that monitors
// cluster health and manages group
// membership functions.
//
func (h *Agent) Start(
	info server.Info,
	opts server.Options,
	lsnReady chan struct{},
	accept func(nc net.Conn),
	logger server.Logger,
) error {

	// To keep the health client fast and its traffic
	// internal-only, we use an bi-directional,
	// in-memory version of a TCP stream.
	cli, srv := lcon.NewBidir(info.MaxPayload * 2)

	rank := info.ServerRank
	beat := time.Second

	cfg := &MembershipCfg{
		MaxClockSkew: time.Second,
		BeatDur:      beat,
		MyRank:       rank,
		CliConn:      cli,
		Logger:       logger,
	}
	h.mship = NewMembership(cfg)

	go func() {
		select {
		case <-lsnReady:
			accept(srv)
		}
	}()
	return h.mship.Start()
}

func (h *Agent) Stop() {
	h.mship.Stop()
}
