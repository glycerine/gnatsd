package health

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/nats-io/go-nats"
)

// sysMemberPrefix creates a namespace
// for system cluster membership communication.
// This prefix aims to avoid collisions
// with user-level topics. Only system
// processes / internal clients should
// write to these topics, but everyone
// is welcome to listen on them.
//
// note: `_nats` is for now, can easily
// changed to be `_SYS` later once
// we're sure everything is working.
//
const sysMemberPrefix = "_nats.cluster.members."

// leaseTime is the minimum time the
// leader is elected for.
var leaseTime = time.Second * 10

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

// NOTE: Use tls scheme for TLS, e.g. nats-sub -s tls://demo.nats.io:4443 foo
func usage() {
	log.Fatalf("Usage: vc host:port\n")
}

func printMsg(m *nats.Msg, i int) {
	fmt.Printf("\n")
	log.Printf("[#%d] Received on [%s]: '%s'\n", i, m.Subject, string(m.Data))
}

// Membership tracks the nats server cluster
// membership, issuing health checks and
// choosing a leader.
type Membership struct {
	Cfg      MembershipCfg
	CurLead  Bchan
	CurGroup Bchan

	elec leadHolder
	nc   *nats.Conn

	subjAllCall     string
	subjMemberLost  string
	subjMemberAdded string
	subjMembership  string

	rcQuit   chan bool
	done     chan bool
	mu       sync.Mutex
	stopping bool
}

func NewMembership(cfg *MembershipCfg) *Membership {
	return &Membership{
		Cfg:      *cfg,
		CurLead:  *NewBchan(1),
		CurGroup: *NewBchan(1),

		rcQuit: make(chan bool),
		done:   make(chan bool),
	}
}

// leadHolder holds who is the current leader,
// and what their lease is. Used to synchronize
// access between various goroutines.
type leadHolder struct {
	mu   sync.Mutex
	sloc ServerLoc
}

func (e *leadHolder) getLeader() ServerLoc {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sloc
}

// we copy sloc and store it
func (e *leadHolder) setLeader(sloc *ServerLoc) {
	if sloc == nil {
		e.sloc = ServerLoc{}
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sloc = *sloc
}

func (e *leadHolder) getLeaderAsBytes() []byte {
	lead := e.getLeader()
	by, err := json.Marshal(&lead)
	panicOn(err)
	return by
}

// Stop blocks until the Membership goroutine
// acknowledges the shutdown request.
func (m *Membership) Stop() {
	m.mu.Lock()
	if m.stopping {
		m.mu.Unlock()
		return
	}
	m.stopping = true
	m.mu.Unlock()
	close(m.rcQuit)
	<-m.done
}

func (m *Membership) Start() error {
	//p("Membership.Start() running")
	m.Cfg.SetDefaults()

	nc, err := m.setupNatsClient()
	if err != nil {
		return err
	}
	m.nc = nc
	go m.start(nc)
	return nil
}

func (m *Membership) start(nc *nats.Conn) {
	//p("Membership.start(nc=%p) running", nc)

	// history
	history := NewRingBuf(10)

	log.Printf("Listening on [%s]\n", m.subjAllCall)
	log.SetFlags(log.LstdFlags)

	var prev, cur *pongCollector
	prevCount, curCount := 0, 0
	var curMember, prevMember *members
	var curLead *ServerLoc

	// do an initial allcall() to discover any
	// current leader.
	log.Printf("init: doing initial allcall to discover any existing leader...")
	prev, err := allcall(nc, m.subjAllCall, &m.elec)
	panicOn(err)

	p("m.Cfg.BeatDur = '%v'", m.Cfg.BeatDur)
	select {
	case <-time.After(m.Cfg.BeatDur):
	case <-m.rcQuit:
		close(m.done)
		return
	}

	prevCount, prevMember = prev.unsubAndGetSet()

	history.Append(prevMember)

	now := time.Now()

	firstSeenLead := m.elec.getLeader()
	xpire := firstSeenLead.LeaseExpires

	limit := xpire.Add(m.Cfg.MaxClockSkew)
	if !xpire.IsZero() && limit.After(now) {
		log.Printf("init: after one heartbeat, we detect current leader '%s' of rank %v with lease good for '%v' until expiration + maxClockSkew=='%v'",
			firstSeenLead.Id, firstSeenLead.Rank, limit.Sub(now), limit,
		)
	} else {
		log.Printf("init: after one heartbeat, no leader found. waiting for a full leader lease term of '%s' to expire...", m.Cfg.LeaseTime)

		select {
		case <-time.After(m.Cfg.LeaseTime):
		case <-m.rcQuit:
			close(m.done)
			return
		}
	}

	// prev responses should be back by now.
	var expired bool
	var prevLead *ServerLoc
	var nextLeadReportTm time.Time

	for {
		// NB: replies to an allcall can/will change what the current leader is in elec.
		cur, err = allcall(nc, m.subjAllCall, &m.elec)
		panicOn(err)

		select {
		case <-time.After(m.Cfg.BeatDur):
		case <-m.rcQuit:
			close(m.done)
			return
		}
		lastSeenLead := m.elec.getLeader()

		// cur responses should be back by now
		// and we can compare prev and cur.
		curCount, curMember = cur.unsubAndGetSet()

		history.Append(curMember)

		now = time.Now()
		expired, curLead = curMember.leaderLeaseExpired(
			now, leaseTime, &lastSeenLead, m.Cfg.MaxClockSkew,
		)

		// tell pong
		m.elec.setLeader(curLead)

		loc, _ := nc.ServerLocation()
		if loc != nil {
			loc.Rank = m.Cfg.MyRank
			if loc.Id == curLead.Id {
				if now.After(nextLeadReportTm) || prevLead == nil || prevLead.Id != curLead.Id {
					left := curLead.LeaseExpires.Sub(now)
					log.Printf("I am LEAD, my Id: '%s', rank %v. lease expires:'%s' in '%s'", loc.Id, loc.Rank, curLead.LeaseExpires, left)
					nextLeadReportTm = now.Add(left).Add(time.Second)
				}
			} else {
				if prevLead != nil && prevLead.Id == loc.Id {
					log.Printf("I am no longer lead, new LEAD is '%s', rank %v. lease expires:'%s' in '%s'", curLead.Id, curLead.Rank, curLead.LeaseExpires, curLead.LeaseExpires.Sub(now))

				}
			}
		}

		lost := setDiff(prevMember, curMember, curLead)
		gained := setDiff(curMember, prevMember, curLead)
		same := setsEqual(prevMember, curMember)

		if same {
			// nothing more to do.
			// This is the common case when nothing changes.
		} else {
			lostBytes := lost.mustJsonBytes()
			if !lost.setEmpty() {
				nc.Publish(m.subjMemberLost, lostBytes)
				// ignore errors on purpose, don't crash mid-health-report.
			}
			gainedBytes := gained.mustJsonBytes()
			if !gained.setEmpty() {
				nc.Publish(m.subjMemberAdded, gainedBytes)
				// ignore errors on purpose
			}
		}
		if curCount < prevCount {
			log.Printf("\n\n\n --------  ARG PAGE PAGE PAGE!! we went down a server, from %v -> %v. lost: '%s'\n\n\n", prevCount, curCount, lost)

		} else if curCount > prevCount {
			log.Printf("\n\n\n +++++++  MORE ROBUSTNESS GAINED; we went from %v -> %v. gained: '%s'\n\n\n", prevCount, curCount, gained)

		}

		if expired {
			curBytes := curMember.mustJsonBytes()
			nc.Publish(m.subjMembership, curBytes)
		}

		// done with compare, now loop
		prev = cur
		prevCount = curCount
		prevMember = curMember
		prevLead = curLead
	}
}

func pong(nc *nats.Conn, subj string, msg []byte) {
	err := nc.Publish(subj, msg)
	panicOn(err)
	err = nc.Flush()
	panicOn(err)
}

// allcall makes a new inbox and pings everyone
// to reply to that inbox.
//
// The ping consists of sending the ServerLoc
// of the current leader, which provides lease
// and full contact info for the leader.
//
// This gives a full
// round-trip connectivity check while filtering
// out concurrent checks from other nodes, so
// we can get an accurate count.
//
func allcall(
	nc *nats.Conn,
	subjAllCall string,
	elec *leadHolder,

) (*pongCollector, error) {

	inbox := nats.NewInbox()

	pc := &pongCollector{
		inbox: inbox,
		t0:    time.Now(),
		from:  newMembers(),
	}

	s, err := nc.Subscribe(inbox, pc.receivePong)

	pc.mu.Lock()
	pc.sub = s
	pc.mu.Unlock()

	if err != nil {
		return nil, err
	}

	// allcall broadcasts the current leader + lease
	leadby := elec.getLeaderAsBytes()
	err = nc.PublishRequest(subjAllCall, inbox, leadby)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

// pongCollector collects the responses
// for a single ping (allcall) request.
type pongCollector struct {
	inbox   string
	t0      time.Time
	replies int
	from    *members
	mu      sync.Mutex
	sub     *nats.Subscription
}

// acumulate pong responses
func (pc *pongCollector) receivePong(msg *nats.Msg) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.replies++

	var loc ServerLoc
	err := loc.fromBytes(msg.Data)
	panicOn(err)

	pc.from.Mem[loc.Id] = &loc
}

// unsubAndGetSet unsubscribes and returns the count and set so far.
func (pc *pongCollector) unsubAndGetSet() (int, *members) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.sub.Unsubscribe()

	pc.from.clearLeaderAndLease()
	return pc.replies, pc.from
}

// leaderLeaseExpired evaluates the lease as of now,
// and returns the leader or best candiate. Returns
// expired == true if any prior leader lease has
// lapsed. In this case we return the best new
// leader with its IsLeader bit set and its
// LeaseExpires set to now + lease.
//
// If expired == false then the we return
// the current leader in lead.
//
// PRE: there are only 0 or 1 leaders in m.Mem
//      who have a non-zero LeaseExpires field.
//
// If m.Mem is empty, we return (true, nil).
//
func (m *members) leaderLeaseExpired(
	now time.Time,
	leaseLen time.Duration,
	prevLead *ServerLoc,
	maxClockSkew time.Duration,

) (expired bool, lead *ServerLoc) {

	if prevLead.LeaseExpires.Add(maxClockSkew).After(now) {
		// honor the leases until they expire
		return false, prevLead
	}

	if len(m.Mem) == 0 {
		return false, prevLead
	}

	// INVAR: any lease has expired.

	var sortme []*ServerLoc
	for _, v := range m.Mem {
		sortme = append(sortme, v)
	}
	m.clearLeaderAndLease()

	sort.Sort(byRankThenId(sortme))
	lead = sortme[0]
	lead.IsLeader = true
	lead.LeaseExpires = now.Add(leaseLen).UTC()

	return true, lead
}

func (m *members) clearLeaderAndLease() {
	for _, v := range m.Mem {
		v.IsLeader = false
		v.LeaseExpires = time.Time{}
	}
}

type byRankThenId []*ServerLoc

func (p byRankThenId) Len() int      { return len(p) }
func (p byRankThenId) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Less must be stable and computable locally yet
// applicable globally: it is how we choose a leader
// in a stable fashion.
func (p byRankThenId) Less(i, j int) bool {
	itm := p[i].LeaseExpires.UnixNano()
	jtm := p[j].LeaseExpires.UnixNano()
	if itm != jtm {
		return itm > jtm // we want an actual time to sort before a zero-time.
	}
	if p[i].Rank != p[j].Rank {
		return p[i].Rank < p[j].Rank
	}
	if p[i].Id != p[j].Id {
		return p[i].Id < p[j].Id
	}
	if p[i].Host != p[j].Host {
		return p[i].Host < p[j].Host
	}
	return p[i].Port < p[j].Port
}

func (m *Membership) setupNatsClient() (*nats.Conn, error) {
	discon := func(nc *nats.Conn) {
		// we will crash with "nats: invalid connection"
		// Fine if we are in process, but generally
		// we should just let the client auto-reconnect.
		//loc, err := nc.ServerLocation()
		//panicOn(err)
		log.Printf("Disconnected from nats!")
	}
	optdis := nats.DisconnectHandler(discon)
	norand := nats.DontRandomize()

	recon := func(nc *nats.Conn) {
		loc, err := nc.ServerLocation()
		panicOn(err)
		loc.Rank = m.Cfg.MyRank
		log.Printf("Reconnect to nats!: loc = '%s'", loc)
	}
	optrecon := nats.ReconnectHandler(recon)

	opts := []nats.Option{optdis, optrecon, norand}
	if m.Cfg.CliConn != nil {
		opts = append(opts, nats.Dialer(&m.Cfg))
	}

	nc, err := nats.Connect(m.Cfg.NatsUrl, opts...)
	if err != nil {
		log.Fatalf("Can't connect: %v\n", err)
	}

	loc, err := nc.ServerLocation()
	if err != nil {
		return nil, err
	}
	loc.Rank = m.Cfg.MyRank
	log.Printf("HELLOWORLD: I am '%s' at '%v:%v'. rank %v", loc.Id, loc.Host, loc.Port, loc.Rank)

	m.subjAllCall = sysMemberPrefix + "allcall"
	m.subjMemberLost = sysMemberPrefix + "lost"
	m.subjMemberAdded = sysMemberPrefix + "added"
	m.subjMembership = sysMemberPrefix + "list"

	nc.Subscribe(m.subjAllCall, func(msg *nats.Msg) {
		loc, err := nc.ServerLocation()
		panicOn(err)
		loc.Rank = m.Cfg.MyRank

		// allcall broadcasts the leader
		var lead ServerLoc
		err = lead.fromBytes(msg.Data)
		panicOn(err)

		if lead.Id != "" && !lead.LeaseExpires.IsZero() {
			m.elec.setLeader(&lead)

			if loc.Id == lead.Id {
				loc.IsLeader = true
				loc.LeaseExpires = lead.LeaseExpires
			} else {
				loc.IsLeader = false
				loc.LeaseExpires = time.Time{}
			}
		}

		hp, err := json.Marshal(loc)
		panicOn(err)
		pong(nc, msg.Reply, hp)
	})

	/* reporting */
	nc.Subscribe(m.subjMemberLost, func(msg *nats.Msg) {
		//printMsg(msg, -1)
	})

	nc.Subscribe(m.subjMemberAdded, func(msg *nats.Msg) {
		//printMsg(msg, -1)
	})

	nc.Subscribe(m.subjMembership, func(msg *nats.Msg) {
		//printMsg(msg, -1)
	})

	return nc, nil
}
