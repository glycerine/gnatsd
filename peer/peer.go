package peer

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"encoding/json"
	"github.com/glycerine/hnatsd/health"
	"github.com/glycerine/hnatsd/logger"
	"github.com/glycerine/hnatsd/server"
	"github.com/glycerine/nats"

	"github.com/glycerine/bchan"
	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b"
	"github.com/glycerine/hnatsd/swp"
	"github.com/glycerine/idem"

	"github.com/glycerine/hnatsd/peer/gserv"

	"github.com/glycerine/hnatsd/peer/api"
	tun "github.com/glycerine/sshego"
)

var utclog *log.Logger

func init() {
	utclog = log.New(os.Stderr, "", log.LUTC|log.LstdFlags|log.Lmicroseconds)
}

type LeadAndFollowList struct {
	Members []health.AgentLoc
	LeadID  string `json:"LeadID"`
	MyID    string
}

// Peer serves as a member of a
// replication cluster. One peer
// will be elected lead. The others
// will be followers. All peers
// will run a background receive
// session.
type Peer struct {
	mut        sync.Mutex
	followSess *swp.Session

	cmdflags []string
	serv     *server.Server

	Halt *idem.Halter

	LeadAndFollowBchan *bchan.Bchan
	MemberGainedBchan  *bchan.Bchan
	MemberLostBchan    *bchan.Bchan

	natsURL string

	subjMembership  string
	subjMemberLost  string
	subjMemberAdded string
	subjBcastGet    string
	subjBcastSet    string

	loc health.AgentLoc
	nc  *nats.Conn

	plog       server.Logger
	serverOpts *server.Options
	clientOpts *[]nats.Option

	LeadStatus leadFlag
	saver      *BoltSaver

	GservCfg *gserv.ServerConfig

	grpcAddr     string
	internalPort int

	Whoami string // as a host

	SshClientLoginUsername        string
	SshClientPrivateKeyPath       string
	SshClientClientKnownHostsPath string

	SshdReady                    chan bool
	SshClientAllowsNewSshdServer bool
	TestAllowOneshotConnect      bool

	// lock mut before reading
	lastSeenInternalPortAloc map[string]health.AgentLoc

	SkipEncryption bool
}

type leadFlag struct {
	amLead bool
	cs     clusterStatus
	mut    sync.Mutex
}

func has(haystack []string, needle string) bool {
	for i := range haystack {
		if haystack[i] == needle {
			return true
		}
	}
	return false
}

// NewPeer should be given the same cmdflags
// as a hnatsd/gnatsd process.
//
// "-routes=nats://localhost:9229 -cluster=nats://localhost:9230 -p 4223"
//
// We auto-append "-health" if not provided, since that is
// essential for our peering network.
//
// Each node needs its own -cluster address and -p port, and
// to form a cluster, the -routes of subsequent nodes
// need to point at one of the -cluster of an earlier started node.
//
// After NewPeer() and before calling Start(), the
// Peer.SetGrpcPorts() method should be called to establish
// which port(s) to listen on.
//
func NewPeer(args, whoami string) (*Peer, error) {

	saver, err := NewBoltSaver(whoami+".boltdb", whoami)
	if err != nil {
		return nil, err
	}

	argv := strings.Fields(args)
	// ensure -health is given
	if !has(argv, "-health") {
		argv = append(argv, "-health")
	}

	r := &Peer{
		cmdflags:                 argv,
		Halt:                     idem.NewHalter(),
		LeadAndFollowBchan:       bchan.New(2),
		MemberGainedBchan:        bchan.New(2),
		MemberLostBchan:          bchan.New(2),
		saver:                    saver,
		Whoami:                   whoami,
		SshdReady:                make(chan bool),
		lastSeenInternalPortAloc: make(map[string]health.AgentLoc),

		// MyID will be fixed in StartBackgroundSshdRecv(),
		// so empty string suffices for now:
		GservCfg: gserv.NewServerConfig(""),
	}
	serv, opts, err := hnatsdMain(argv)
	if err != nil {
		return nil, err
	}

	r.serverOpts = opts
	r.serv = serv

	// log controls
	const colors = false
	const micros, pid = true, true
	const trace = false
	//const debug = true
	const debug = false

	r.plog = logger.NewStdLogger(micros, debug, trace, colors, pid, log.LUTC)

	return r, nil
}

func (peer *Peer) GetGrpcPorts() (xport, iport int) {
	return peer.GservCfg.ExternalLsnPort,
		peer.GservCfg.InternalLsnPort
}

func (peer *Peer) SetGrpcPorts(xport, iport int, xhost string) {
	peer.GservCfg.ExternalLsnPort = xport
	peer.GservCfg.InternalLsnPort = iport
	peer.GservCfg.ExternalHost = xhost
}

// Start launches an embedded
// gnatsd instance in the background.
func (peer *Peer) Start() error {
	go peer.serv.Start()

	// and monitor the leadership
	// status so we know if we
	// are pushing or receiving
	// checkpoints.
	err := peer.setupNatsClient()
	//p("%v peer.Start() done with peer.setupNatsClient() err='%v'", peer.loc.ID, err)

	if err != nil {
		utclog.Printf("warning: not starting background peer goroutine, as we got err from setupNatsCli: '%v'", err)
		return err
	}

	// get lead/follow situation
	select {
	case <-time.After(120 * time.Second):
		panic("problem: no lead/follow status after 2 minutes")
	case list := <-peer.LeadAndFollowBchan.Ch:
		peer.LeadAndFollowBchan.BcastAck()
		laf := list.(*LeadAndFollowList)

		cs, myFollowSubj := list2status(laf)
		utclog.Printf("peer.Start(): we have clusterStatus: '%s'", &cs)

		peer.StartBackgroundCheckpointdRecv(laf.MyID, myFollowSubj)
	}
	return nil
}

// Stop shutsdown the embedded gnatsd
// instance.
func (peer *Peer) Stop() {
	//p("%s peer.Stop() invoked, shutting down...", peer.loc.ID)
	if peer != nil {
		sessF := peer.GetFollowSess()
		if sessF != nil {
			peer.SetFollowSess(nil)
			//p("%s peer.Stop() is invoking sessF.Close() and Stop()", peer.loc.ID)
			// unblock Session.Read() from sessF.RecvFile()
			sessF.Close()
			sessF.Stop()
		}

		if peer.nc != nil {
			peer.nc.Close()
			peer.nc = nil
		}
		if peer.serv != nil {
			peer.serv.Shutdown()
			peer.serv = nil
		}
		if peer.GservCfg != nil {
			peer.GservCfg.Stop()
		}
		peer.Halt.ReqStop.Close()
		select {
		case <-peer.Halt.Done.Chan:
		case <-time.After(5 * time.Second):
		}
	}
}

func (peer *Peer) setupNatsClient() error {

	peer.natsURL = fmt.Sprintf("nats://%v:%v", peer.serverOpts.Host, peer.serverOpts.Port)
	//p("setupNatsClient() is trying url '%s'", peer.natsURL)
	recon := nats.MaxReconnects(-1) // retry forevever.
	norand := nats.DontRandomize()

	opts := []nats.Option{recon, norand}
	var nc *nats.Conn
	var err error
	try := 0
	tryLimit := 20

	for {
		nc, err = nats.Connect(peer.natsURL, opts...)
		if err == nil {
			break
		}
		if try < tryLimit {
			//p("nats.Connect() failed at try %v, with err '%v'. trying again after 1 second.", try, err)
			time.Sleep(time.Second)
			continue
		}
		if err != nil {
			msg := fmt.Errorf("Can't connect to "+
				"nats on url '%s': %v",
				peer.natsURL,
				err)
			panic(msg)
			return msg
		}
	}
	peer.clientOpts = &opts
	peer.nc = nc
	var loc *nats.ServerLoc
	for {
		loc, err = peer.nc.ServerLocation()
		if err != nil {
			//p("peer.nc.ServerLocation() returned error '%v'", err)
			return err
		}
		if loc == nil {
			//p("got nil loc, waiting for reconnect")
			time.Sleep(3 * time.Second)
		} else {
			//p("got loc = %p, ok", loc)
			break
		}
	}
	// first assignment to peer.loc is here.
	peer.loc = *(natsLocConvert(loc))

	peer.subjMembership = health.SysMemberPrefix + "list"
	peer.subjMemberLost = health.SysMemberPrefix + "lost"
	peer.subjMemberAdded = health.SysMemberPrefix + "added"
	peer.subjBcastGet = "bcast_get"
	peer.subjBcastSet = "bcast_set"

	// BCAST GET handler
	getScrip, err := nc.Subscribe(peer.subjBcastGet, func(msg *nats.Msg) {
		// see bcast.go
		err := peer.ServerHandleBcastGet(msg)
		panicOn(err)
	})
	panicOn(err)
	getScrip.SetPendingLimits(-1, -1)

	// BcastSet
	setScrip, err := nc.Subscribe(peer.subjBcastSet, func(msg *nats.Msg) {
		var bsr api.BcastSetRequest
		bsr.UnmarshalMsg(msg.Data)
		utclog.Printf("peer received subjBcastSet(fromID='%s') for key '%s'",
			string(bsr.Ki.Key), bsr.FromID)

		var reply api.BcastSetReply

		err := peer.LocalSet(bsr.Ki)
		if err != nil {
			utclog.Printf("peer.LocalSet(key='%s') returned error '%v'", string(bsr.Ki.Key), err)
			reply.Err = err.Error()
		}
		mm, err := reply.MarshalMsg(nil)
		panicOn(err)
		err = nc.Publish(msg.Reply, mm)
		panicOn(err)
	})
	panicOn(err)
	setScrip.SetPendingLimits(-1, -1)

	// reporting
	nc.Subscribe(peer.subjMemberLost, func(msg *nats.Msg) {
		utclog.Printf("peer recevied subjMemberLost: "+
			"Received on [%s]: '%s'",
			msg.Subject,
			string(msg.Data))

		var laf LeadAndFollowList
		json.Unmarshal(msg.Data, &laf)
		laf.MyID = peer.loc.ID

		peer.MemberLostBchan.Bcast(&laf)
	})

	// reporting
	nc.Subscribe(peer.subjMemberAdded, func(msg *nats.Msg) {
		utclog.Printf("peer recevied subjMemberAdded: Received on [%s]: '%s'",
			msg.Subject, string(msg.Data))

		var laf LeadAndFollowList
		json.Unmarshal(msg.Data, &laf)

		peer.mut.Lock()
		laf.MyID = peer.loc.ID // prev read, race with line 379
		peer.mut.Unlock()

		peer.MemberGainedBchan.Bcast(&laf)
	})

	// reporting
	// problem: reporting every 5msec, not good
	nc.Subscribe(peer.subjMembership, func(msg *nats.Msg) {
		/*utclog.Printf("peer received subjMembership: "+
		"Received on [%s]: '%s'",
		msg.Subject,
		string(msg.Data))
		*/
		var laf LeadAndFollowList
		json.Unmarshal(msg.Data, &laf)
		peer.mut.Lock()
		laf.MyID = peer.loc.ID

		// update our peer.loc too, so it is current/accurate.
		for _, v := range laf.Members {
			if v.ID == peer.loc.ID {
				peer.loc = v // race here, vs line 358
				break
			}
		}
		peer.mut.Unlock()

		peer.LeadAndFollowBchan.Bcast(&laf)
	})

	// request everyone's grpc and internal ports
	peer.StartPeriodicClusterAgentLocQueries()

	// queries to the peer list - for grpc ext+internal ports
	nc.Subscribe(peer.loc.ID+".>", func(msg *nats.Msg) {

		subSubject := msg.Subject[len(peer.loc.ID)+1:]

		// local and grab the info we need to share
		peer.mut.Lock()
		aloc := peer.loc
		externalHost := peer.GservCfg.ExternalHost
		internalHost := peer.GservCfg.InternalHost
		externalPort := peer.GservCfg.ExternalLsnPort
		internalPort := peer.GservCfg.InternalLsnPort
		peer.mut.Unlock()

		/* reporting every 10msec problem:
		utclog.Printf("peer '%s' received on subSubject %s: '%s', where I have peer.loc='%s'",
			aloc.ID,
			subSubject,
			string(msg.Data),
			&aloc,
		)
		*/
		switch subSubject {
		case "grpc-port-query":
			aloc.Grpc.External.Host = externalHost
			aloc.Grpc.External.Port = externalPort
			aloc.Grpc.Internal.Host = internalHost
			aloc.Grpc.Internal.Port = internalPort

			err = nc.Publish(msg.Reply, []byte(aloc.String()))
			if err != nil {
				utclog.Printf("warning: '%s' publish to '%s' got error '%v'",
					subSubject,
					msg.Reply, err)
			}
		default:
			panic(fmt.Sprintf("unknown subSubject '%s'", subSubject))
		}
	})

	return nil
}

func agentLoc2RecvCpSubj(a health.AgentLoc) string {
	return fmt.Sprintf("recv-chkpt;id:%v;nats-host:%v;nats-client-port:%v;rank:%v;pid:%v;grpc.ext=%v:%v",
		a.ID, a.NatsHost, a.NatsClientPort, a.Rank, a.Pid, a.Grpc.External.Host, a.Grpc.External.Port)
}

var ErrShutdown = fmt.Errorf("shutting down")

const ignoreSlowConsumerErrors = true
const skipTLS = true

var ErrAmFollower = fmt.Errorf("LeadTransferCheckpoint error: I am follower, not transmitting checkpoint")

var ErrAmLead = fmt.Errorf("error: I am lead")
var ErrNoFollowers = fmt.Errorf("error: no followers")

type Saver interface {
	WriteKv(key, val []byte, timestamp time.Time) error
}

// LeadTransferCheckpoint is the raison d'etre of
// the peer libary. This function is called
// called when we've just generated
// a checkpoint and need to propagate
// it out to our followers. It is safe to be
// called by followers, but that should
// be avoided since the background peer
// will already be checkpointing when
// it receives data from the lead.
//
// The first time the lead calls this,
// it should be with chkptData == nil.
// This will signal to a new lead
// that it should get
// the most recent checkpoint from
// the cluster (including
// ourselves) so that new leads taking
// over don't ignore the very
// recent state from any previous lead.
//
func (peer *Peer) LeadTransferCheckpoint(key, chkptData []byte, when time.Time) error {
	utclog.Printf("top of LeadTransferCheckpoint")
	var list interface{}
	select {
	case <-peer.Halt.ReqStop.Chan:
		// shutting down.
		utclog.Printf("shutting down on request from peer.Halt.ReqStop.Chan")
		return ErrShutdown

	case list = <-peer.LeadAndFollowBchan.Ch:
		peer.LeadAndFollowBchan.BcastAck()
	}

	laf := list.(*LeadAndFollowList)

	cs, _ := list2status(laf)
	utclog.Printf("LeadTransferCheckpoint(): we have clusterStatus: '%s'", &cs)

	if len(chkptData) == 0 {
		// the first time, we get the latest from
		// the cluster. Whether follow or lead
		// it doesn't matter.
		utclog.Printf("LeadTransferCheckpoint sees len chkptData of zero, doing GetLatest on key '%s'", string(key))

		ki, err := peer.GetLatest(key, true)
		if err != nil {
			return err
		}

		utclog.Printf("LeadTransferCheckpoint got back from GetLatest ki.Key='%s'; ki.When='%s'; ki.Blake2b='%x'. len(ki.Val)=%v. ki.Who='%s'", string(ki.Key), ki.When, ki.Blake2b, len(ki.Val), ki.Who)

		// checkpoint it... unless it was from ourselves!
		if ki.Who != peer.loc.ID && ki.Who != peer.GservCfg.SshegoCfg.EmbeddedSSHdHostDbPath {
			// checkpoint it

			utclog.Printf("ki.Who='%s' != peer.loc.ID='%s', doing LocalSet(ki.Key='%s'. checksum='%x')", ki.Who, peer.loc.ID, string(ki.Key), ki.Blake2b)
			err = peer.LocalSet(ki)
			if err != nil {
				return err
			}
		} else {
			utclog.Printf("ki.Who='%s' == peer.loc.ID='%s' or peer.GservCfg.SshegoCfg.EmbeddedSSHdHostDbPath='%s', so skipping LocalSet().doing LocalSet(ki.Key='%s'. checksum='%x')", ki.Who, peer.loc.ID, peer.GservCfg.SshegoCfg.EmbeddedSSHdHostDbPath, string(ki.Key), ki.Blake2b)
		}
		return nil
	}

	if laf.MyID != laf.LeadID {
		// follower, don't transmit checkpoints, should not really even
		// be here... but might be a delay in recognizing that.
		return ErrAmFollower
	}

	utclog.Printf("MyID:'%v' I AM LEAD. I have %v follows.",
		laf.MyID, len(cs.follow))

	// Since we are lead, we send this checkpoint out.
	// BcastSet does the LocalSet for us.
	return peer.BcastSet(&api.KeyInv{Key: key, Val: chkptData, When: when})
}

func (peer *Peer) AmFollow() bool {
	select {
	case list := <-peer.LeadAndFollowBchan.Ch:
		peer.LeadAndFollowBchan.BcastAck()
		laf := list.(*LeadAndFollowList)
		if laf.MyID == laf.LeadID {
			return false
		}
	case <-peer.Halt.ReqStop.Chan:
		return true
	}
	return true
}

func list2status(laf *LeadAndFollowList) (cs clusterStatus, myFollowSubj string) {

	if laf.LeadID == laf.MyID {
		cs.amLead = true
	}

	// pull out the lead/follow sessions as nats subjects
	follow := []health.AgentLoc{}
	followSubj := []string{}
	var leadSubj string // for when I am lead
	for i := range laf.Members {
		if laf.Members[i].ID == laf.LeadID {
			// lead
			leadSubj = agentLoc2RecvCpSubj(laf.Members[i])
			cs.lead = &peerDetail{subj: leadSubj, loc: laf.Members[i]}

			// lead should have myfollowSubj set too, so that
			// we start the background receiver correctly should
			// the lead become a follower.
			cs.myfollowSubj = leadSubj
		} else {
			// followers
			follow = append(follow, laf.Members[i])

			fsj := agentLoc2RecvCpSubj(laf.Members[i])
			followSubj = append(followSubj, fsj)
			cs.follow = append(cs.follow, &peerDetail{subj: fsj, loc: laf.Members[i]})
			if laf.Members[i].ID == laf.MyID {
				cs.myfollowSubj = fsj
			}
		}
	}
	return cs, cs.myfollowSubj
}

type peerDetail struct {
	loc  health.AgentLoc
	subj string
}

func (d peerDetail) String() string {
	//return fmt.Sprintf(`%s`, &(d.loc)) // display in JSON format
	//return fmt.Sprintf(`peerDetail={subj:"%s", loc:%s}`, d.subj, &(d.loc))
	return fmt.Sprintf(`peerDetail={subj:"%s"}`, d.subj)
}

type clusterStatus struct {
	follow       []*peerDetail
	lead         *peerDetail
	myfollowSubj string
	amLead       bool
}

func (cs clusterStatus) String() string {
	s := fmt.Sprintf(" myfollowSubj:'%s'\n lead[me:%v]: %s\n",
		cs.myfollowSubj, cs.amLead, cs.lead)
	for i := range cs.follow {
		s += fmt.Sprintf("  follow %v[me:%v]: %s\n",
			i, cs.follow[i].subj == cs.myfollowSubj, cs.follow[i])
	}
	return s
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ==================================
// all nodes run a peer network
// in the background, keeping a
// session in Listen for checkpoints.
// ==================================

// StartBackroundCheckpointdRecv will keep a peer
// running in the background and
// always accepting and writing checkpoints (as
// long as we are not lead when they are received).
// Track these by their timestamps, and if we have a new one
// (recognized by a more recent timestamp), then
// save it to disk (this dedups if we get multiples of the same).
//
// The host address bound will be taken from peer.serverOpts.Host
// which corresponds to the --addr in the nats options.
//
// After NewPeer() and before calling this routine, the
// Peer.SetGrpcPorts() method should be called to establish
// which port(s) to listen on.
//
func (peer *Peer) StartBackgroundCheckpointdRecv(myID, myFollowSubj string) {
	utclog.Printf("beginning StartBackgroundCheckpointdRecv(myID='%s', "+
		"myFollowSubj='%s'). peer.SkipEncryption=%v",
		myID, myFollowSubj, peer.SkipEncryption)

	if peer.GservCfg.ExternalLsnPort == 0 {
		portx, lsnx := getAvailPort()
		lsnx.Close()
		peer.GservCfg.ExternalLsnPort = portx
		utclog.Printf("warning: detected peer.GservCfg.ExternalLsnPort == 0. You should call peer.SetGrpcPrts() before invoking StartBackgroundSsdhRecv. We picked a random port, %v, for grpc external listen", portx)
	}

	if peer.GservCfg.InternalLsnPort == 0 {
		if !peer.SkipEncryption {
			// really only need internal port for SSH...
			// but for TLS it won't hurt to have
			// an unused internal port set.
			porti, lsni := getAvailPort()
			lsni.Close()
			peer.GservCfg.InternalLsnPort = porti
			utclog.Printf("warning: detected peer.GservCfg.InternalLsnPort == 0. You should call peer.SetGrpcPrts() before invoking StartBackgroundSsdhRecv. We picked a random port, %v, for grpc internal listen", porti)
		}
	}

	go func() {
		defer func() {
			peer.Halt.ReqStop.Close()
			peer.Halt.Done.Close()
			utclog.Printf("StartBackgroundCheckpointdRecv(myID='%s', "+
				"myFollowSubj='%s') has shutdown.",
				myID, myFollowSubj)
		}()

		// Start grpc server endpoint.
		// It writes to boltdb upon receipt
		// of a checkpoint file; and serves
		// files upon demand.

		// set up the GservCfg
		peer.GservCfg.MyID = myID
		if peer.GservCfg.ExternalHost == "" {
			peer.GservCfg.ExternalHost = peer.serverOpts.Host
		}

		if peer.GservCfg.InternalHost == "" {
			peer.GservCfg.InternalHost = "127.0.0.1"
		}

		peer.GservCfg.SkipEncryption = peer.SkipEncryption
		peer.GservCfg.SshegoCfg = &tun.SshegoConfig{
			Username:                peer.serverOpts.Username,
			TestAllowOneshotConnect: peer.TestAllowOneshotConnect,
		}

		//p("%s StartBackgroundCheckpointdRecv: peer.GservCfg.SkipEncryption = %v", peer.loc.ID, peer.GservCfg.SkipEncryption)

		// fill default SshegoCfg
		cfg := peer.GservCfg.SshegoCfg

		home := os.Getenv("HOME")
		cfg.PrivateKeyPath = home + "/.ssh/id_rsa_nopw"
		cfg.ClientKnownHostsPath = home + "/.ssh/.sshego.cli.known.hosts." + peer.Whoami
		cfg.BitLenRSAkeys = 4096

		// make these unique for each peer by adding Whoami
		cfg.EmbeddedSSHdHostDbPath += ("." + peer.Whoami)

		port2, lsn2 := getAvailPort()
		lsn2.Close()
		cfg.SshegoSystemMutexPort = port2

		peer.grpcAddr = fmt.Sprintf("%v:%v", peer.GservCfg.ExternalHost, peer.GservCfg.ExternalLsnPort)

		peer.BackgroundReceiveBcastSetAndWriteToBolt()

		// will block until server exits:
		peer.GservCfg.StartGrpcServer(peer.saver, peer.SshdReady, myID)
	}()
}

func (peer *Peer) SetFollowSess(sessF *swp.Session) {
	//p("%s SetFollowSess(%p) called.", peer.loc.ID, sessF)
	peer.mut.Lock()
	peer.followSess = sessF
	peer.mut.Unlock()
}
func (peer *Peer) GetFollowSess() (sessF *swp.Session) {
	peer.mut.Lock()
	sessF = peer.followSess
	peer.mut.Unlock()
	return
}

func (peer *Peer) GetPeerList(timeout time.Duration) (*LeadAndFollowList, error) {

	select {
	case <-time.After(timeout):
		return nil, ErrTimedOut
	case list := <-peer.LeadAndFollowBchan.Ch:
		peer.LeadAndFollowBchan.BcastAck()
		laf := list.(*LeadAndFollowList)
		return laf, nil
	}
	return nil, nil
}

func (peer *Peer) WaitForPeerCount(n int, timeout time.Duration) (*LeadAndFollowList, error) {
	toCh := time.After(timeout)
	for {
		select {
		case <-toCh:
			return nil, ErrTimedOut
		case list := <-peer.LeadAndFollowBchan.Ch:
			peer.LeadAndFollowBchan.BcastAck()
			laf := list.(*LeadAndFollowList)
			if len(laf.Members) >= n {
				return laf, nil
			}
			time.Sleep(time.Second)
		}
	}
	return nil, nil
}

func blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	panicOn(err)
	h.Write(by)
	return []byte(h.Sum(nil))
}

func (peer *Peer) GetGrpcAddr() string {
	peer.mut.Lock()
	port := peer.grpcAddr
	peer.mut.Unlock()
	return port
}

func natsLocConvert(loc *nats.ServerLoc) *health.AgentLoc {
	return &health.AgentLoc{
		ID:             loc.ID,
		NatsHost:       loc.Host,
		NatsClientPort: loc.NatsPort,
		Rank:           loc.Rank,
		Pid:            loc.Pid,
	}
}

func (peer *Peer) StartPeriodicClusterAgentLocQueries() {
	go func() {
		toDur := time.Second * 50

		for {
			// every 10 seconds
			select {
			case <-peer.Halt.ReqStop.Chan:
				return
			case <-time.After(10 * time.Second):
			}

			select {
			case list := <-peer.LeadAndFollowBchan.Ch:
				peer.LeadAndFollowBchan.BcastAck()
				laf := list.(*LeadAndFollowList)
				for _, mem := range laf.Members {
					reqsubj := mem.ID + ".grpc-port-query"
					msg, err := peer.nc.Request(reqsubj, nil, toDur)
					if err != nil || msg == nil {
						log.Printf("warning: request for '%s' failed: %v", reqsubj, err)
						continue
					}
					var aloc health.AgentLoc
					err = json.Unmarshal(msg.Data, &aloc)
					panicOn(err)

					// now we have aloc.GrpcPort and aloc.InternalPort
					// for this peer.
					peer.mut.Lock()
					peer.lastSeenInternalPortAloc[mem.ID] = aloc
					utclog.Printf("setting peer.lastSeenInternalPortAloc[mem.ID='%s'] = aloc = %#v", mem.ID, aloc)
					peer.mut.Unlock()
				}
			case <-peer.Halt.ReqStop.Chan:
				return
			}
		}
	}()
}
