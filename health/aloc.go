package health

import (
	"encoding/json"
	"os"
	"time"

	"github.com/glycerine/nats"
)

type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ExtInt conveys the external/internal
// port pairs. Sshd secures external
// ports, and forwards authenticated
// connections to localhost:internal port.
//
type ExtInt struct {
	External HostPort `json:"externalHostPort"`
	Internal HostPort `json:"internalHostPort"`
}

// AgentLoc conveys to interested parties
// the Id and location of one gnatsd
// server in the cluster.
type AgentLoc struct {
	ID             string `json:"serverId"`
	NatsHost       string `json:"natsHost"`
	NatsClientPort int    `json:"natsClientPort"`

	Grpc ExtInt `json:"grpc"`

	// LeaseExpires is zero for any
	// non-leader. For the leader,
	// LeaseExpires tells you when
	// the leaders lease expires.
	LeaseExpires time.Time `json:"leaseExpires"`

	// lower rank is leader until lease
	// expires. Ties are broken by ID.
	// Rank should be assignable on the
	// gnatsd command line with -rank to
	// let the operator prioritize
	// leadership for certain hosts.
	Rank int `json:"rank"`

	// Pid or process id is the only
	// way to tell apart two processes
	// sometimes, if they share the
	// same nats server.
	//
	// Pid is one difference between
	// a nats.ServerLoc and a health.AgentLoc.
	//
	Pid int `json:"pid"`
}

func (s *AgentLoc) String() string {
	by, err := json.Marshal(s)
	panicOn(err)
	return string(by)
}

func (s *AgentLoc) fromBytes(by []byte) error {
	return json.Unmarshal(by, s)
}

func alocEqual(a, b *AgentLoc) bool {
	aless := AgentLocLessThan(a, b)
	bless := AgentLocLessThan(b, a)
	return !aless && !bless
}

func slocEqualIgnoreLease(a, b *AgentLoc) bool {
	a0 := *a
	b0 := *b
	a0.LeaseExpires = time.Time{}
	b0.LeaseExpires = time.Time{}

	aless := AgentLocLessThan(&a0, &b0)
	bless := AgentLocLessThan(&b0, &a0)
	return !aless && !bless
}

// The 2 types (nats.ServerLoc and AgentLoc)
// should be kept in sync.
// We return a brand new &AgentLoc{}
// with contents filled from loc.
func natsLocConvert(loc *nats.ServerLoc) *AgentLoc {
	return &AgentLoc{
		ID:             loc.ID,
		NatsHost:       loc.Host,
		NatsClientPort: loc.NatsPort,
		Rank:           loc.Rank,
		Pid:            os.Getpid(),
	}
}
