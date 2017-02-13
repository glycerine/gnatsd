package health

import (
	"encoding/json"
	"time"

	"github.com/nats-io/go-nats"
)

// compile time check that the ServerLoc
// and nats.ServerLoc are still in sync.
var myCompileTimeSyncCheck ServerLoc = ServerLoc(nats.ServerLoc{})

// ServerLoc conveys to interested parties
// the Id and location of one gnatsd
// server in the cluster.
type ServerLoc struct {
	Id   string `json:"serverId"`
	Host string `json:"host"`
	Port int    `json:"port"`

	// Are we the leader?
	IsLeader bool `json:"leader"`

	// LeaseExpires is zero for any
	// non-leader. For the leader,
	// LeaseExpires tells you when
	// the leaders lease expires.
	LeaseExpires time.Time `json:"leaseExpires"`

	// lower rank is leader until lease
	// expires. Ties are broken by Id.
	// Rank should be assignable on the
	// gnatsd command line with -rank to
	// let the operator prioritize
	// leadership for certain hosts.
	Rank int `json:"rank"`
}

func (s *ServerLoc) String() string {
	by, err := json.Marshal(s)
	panicOn(err)
	return string(by)
}

func (s *ServerLoc) fromBytes(by []byte) error {
	return json.Unmarshal(by, s)
}

func slocEqual(a, b *ServerLoc) bool {
	aless := ServerLocLessThan(a, b)
	bless := ServerLocLessThan(b, a)
	return !aless && !bless
}

func slocEqualIgnoreLease(a, b *ServerLoc) bool {
	a0 := *a
	b0 := *b
	a0.LeaseExpires = time.Time{}
	a0.IsLeader = false
	b0.LeaseExpires = time.Time{}
	b0.IsLeader = false

	aless := ServerLocLessThan(&a0, &b0)
	bless := ServerLocLessThan(&b0, &a0)
	return !aless && !bless
}

// the 2 types should be kept in sync.
// We return a brand new &ServerLoc{}
// with contents filled from loc.
func natsLocConvert(loc *nats.ServerLoc) *ServerLoc {
	return &ServerLoc{
		Id:           loc.Id,
		Host:         loc.Host,
		Port:         loc.Port,
		IsLeader:     loc.IsLeader,
		LeaseExpires: loc.LeaseExpires,
		Rank:         loc.Rank,
	}
}
