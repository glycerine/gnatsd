package health

import (
	"net"
	"time"
)

type MembershipCfg struct {

	// max we allow for clocks to be out of sync.
	// default to 1 second if not set.
	MaxClockSkew time.Duration

	// how often we heartbeat. defaults to 100msec
	// if not set.
	BeatDur time.Duration

	// NatsUrl example "nats://127.0.0.1:4222"
	NatsUrl string

	// defaults to "_nats.cluster.members."
	SysMemberPrefix string

	// LeaseTime is the minimum time the
	// leader is elected for. Defaults to 10 sec.
	LeaseTime time.Duration

	// provide a default until the server gives us rank
	MyRank int

	// optional, if provided we will use this connection on
	// the client side.
	CliConn net.Conn

	// optional, if provided we will use this connection on
	// the server side.
	SrvConn net.Conn
}

func (cfg *MembershipCfg) SetDefaults() {
	if cfg.LeaseTime == 0 {
		cfg.LeaseTime = time.Second * 10
	}
	if cfg.SysMemberPrefix == "" {
		cfg.SysMemberPrefix = "_nats.cluster.members."
	}
	if cfg.BeatDur == 0 {
		cfg.BeatDur = 100 * time.Millisecond
	}
	if cfg.MaxClockSkew == 0 {
		cfg.MaxClockSkew = time.Second
	}
	if cfg.NatsUrl == "" {
		cfg.NatsUrl = "nats://127.0.0.1:4222"
	}
}
