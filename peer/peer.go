package peer

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"
	"github.com/glycerine/hnatsd/health"
	"github.com/glycerine/hnatsd/logger"
	"github.com/glycerine/hnatsd/server"
	"github.com/glycerine/nats"

	"github.com/glycerine/bchan"
	"github.com/glycerine/hnatsd/swp"
	"github.com/glycerine/idem"
)

var mylog *log.Logger

func init() {
	mylog = log.New(os.Stderr, "", log.LUTC|log.LstdFlags|log.Lmicroseconds)
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

	loc *nats.ServerLoc
	nc  *nats.Conn

	plog       server.Logger
	serverOpts *server.Options
	clientOpts *[]nats.Option

	LeadStatus leadFlag
	saver      *BoltSaver
}

type leadFlag struct {
	amLead bool
	cs     clusterStatus
	mut    sync.Mutex
}

func (lf *leadFlag) SetIsLead(val bool, cs clusterStatus) {
	lf.mut.Lock()
	lf.amLead = val
	lf.cs = cs
	lf.mut.Unlock()
}

func (lf *leadFlag) IsLead() (bool, clusterStatus) {
	lf.mut.Lock()
	val := lf.amLead
	cs := lf.cs
	lf.mut.Unlock()
	return val, cs
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
		cmdflags:           argv,
		Halt:               idem.NewHalter(),
		LeadAndFollowBchan: bchan.New(2),
		MemberGainedBchan:  bchan.New(2),
		MemberLostBchan:    bchan.New(2),
		saver:              saver,
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

// Start launches an embedded
// gnatsd instance in the background.
func (peer *Peer) Start() error {
	go peer.serv.Start()

	// and monitor the leadership
	// status so we know if we
	// are pushing or receiving
	// checkpoints.
	err := peer.setupNatsClient()
	p("%v peer.Start() done with peer.setupNatsClient() err='%v'", peer.loc.ID, err)

	if err != nil {
		mylog.Printf("warning: not starting background peer goroutine, as we got err from setupNatsCli: '%v'", err)
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
		mylog.Printf("peer.Start(): we have clusterStatus: '%s'", &cs)

		peer.StartBackgroundRecv(laf.MyID, myFollowSubj)
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
		peer.Halt.ReqStop.Close()
		<-peer.Halt.Done.Chan
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
	peer.loc = loc

	peer.subjMembership = health.SysMemberPrefix + "list"
	peer.subjMemberLost = health.SysMemberPrefix + "lost"
	peer.subjMemberAdded = health.SysMemberPrefix + "added"
	peer.subjBcastGet = "bcast_get"
	peer.subjBcastSet = "bcast_set"

	nc.Subscribe(peer.subjBcastGet, func(msg *nats.Msg) {
		var bgr BcastGetRequest
		bgr.UnmarshalMsg(msg.Data)
		mylog.Printf("peer recevied subjBcastGet for key '%s'",
			string(bgr.Key))

		// are we filtered down to a specific peer request?
		if bgr.Who != "" {
			// yep
			if bgr.Who == peer.saver.whoami {
				p("%s sees peer-specific BcastGet request!", bgr.Who)
			} else {
				p("%s sees peer-specific BcastGet request for '%s' which is not us!", peer.saver.whoami, bgr.Who)
				return
			}
		} else {
			p("bgr.Who was not set...")
		}

		var reply BcastGetReply

		ki, err := peer.LocalGet(bgr.Key, bgr.IncludeValue)
		if err != nil {
			mylog.Printf("peer.LocalGet('%s' returned error '%v'", string(bgr.Key), err)
			reply.Err = err.Error()
		} else {
			reply.Ki = ki
		}
		mm, err := reply.MarshalMsg(nil)
		panicOn(err)
		err = nc.Publish(msg.Reply, mm)
		panicOn(err)
	})

	// BcastSet
	nc.Subscribe(peer.subjBcastSet, func(msg *nats.Msg) {
		var bsr BcastSetRequest
		bsr.UnmarshalMsg(msg.Data)
		mylog.Printf("peer recevied subjBcastSet for key '%s'",
			string(bsr.Ki.Key))

		var reply BcastSetReply

		err := peer.LocalSet(bsr.Ki)
		if err != nil {
			mylog.Printf("peer.LocalSet(key='%s') returned error '%v'", string(bsr.Ki.Key), err)
			reply.Err = err.Error()
		}
		mm, err := reply.MarshalMsg(nil)
		panicOn(err)
		err = nc.Publish(msg.Reply, mm)
		panicOn(err)
	})

	// reporting
	nc.Subscribe(peer.subjMemberLost, func(msg *nats.Msg) {
		mylog.Printf("peer recevied subjMemberLost: "+
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
		mylog.Printf("peer recevied subjMemberAdded: Received on [%s]: '%s'",
			msg.Subject, string(msg.Data))

		var laf LeadAndFollowList
		json.Unmarshal(msg.Data, &laf)
		laf.MyID = peer.loc.ID

		peer.MemberGainedBchan.Bcast(&laf)
	})

	// reporting
	nc.Subscribe(peer.subjMembership, func(msg *nats.Msg) {
		mylog.Printf("peer receved subjMembership: "+
			"Received on [%s]: '%s'",
			msg.Subject,
			string(msg.Data))

		var laf LeadAndFollowList
		json.Unmarshal(msg.Data, &laf)
		laf.MyID = peer.loc.ID

		peer.LeadAndFollowBchan.Bcast(&laf)
	})

	return nil
}

func agentLoc2RecvCpSubj(a health.AgentLoc) string {
	return fmt.Sprintf("recv-chkpt;id:%v;host:%v;port:%v;rank:%v;pid:%v",
		a.ID, a.Host, a.Port, a.Rank, a.Pid)
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

// LeadTransferCheckpoint is called when we've just generated
// a checkpoint and need to propagate it out to our followers.
func (peer *Peer) LeadTransferCheckpoint(chkptData []byte) error {
	//p("top of LeadTransferCheckpoint")
	select {
	case list := <-peer.LeadAndFollowBchan.Ch:
		peer.LeadAndFollowBchan.BcastAck()
		laf := list.(*LeadAndFollowList)

		cs, _ := list2status(laf)
		mylog.Printf("LeadTransferCheckpoint(): we have clusterStatus: '%s'", &cs)

		if laf.MyID != laf.LeadID {
			// follower, don't transmit checkpoints, should not really even
			// be here... but might be a delay in recognizing that.
			return ErrAmFollower
		}

		mylog.Printf("MyID:'%v' I AM LEAD. I have %v follows.", laf.MyID, len(cs.follow))
		peer.LeadStatus.SetIsLead(true, cs)

		if len(cs.follow) == 0 {
			return ErrNoFollowers
		}

		// if we are lead:
		//   if we are newly lead, at startup:
		//      1) poll and recover state from latest checkpoint
		// 2) send checkpoints to followers every so often

		// setup sessions with all followers
		for i := range cs.follow {

			// do pairwise 1:1 transfer for each
			// follower that is not ourselves.

			//p("LeadTransferCheckpoint transferring checkpoint "+"to cs.follow[i].subj='%s'", cs.follow[i].subj)

			sessL, err := swp.SetupRecvStream(
				peer.nc,
				peer.serverOpts.Host,
				peer.serverOpts.Port,
				laf.MyID,
				// We must distinguish our endpoint from
				// that of the lead's recv-chkpt, or else
				// the packets will get mixed up between
				// the two endpoints. Add a prefix that
				// distinguishes the lead that is originating
				// (sending) the checkpoint out.
				"lead-chkpt-origin;"+cs.lead.subj,
				cs.follow[i].subj,
				skipTLS,
				nil,
				ignoreSlowConsumerErrors)

			panicOn(err)

			//p("sessLead = %p", sessL)

			path := "checkpoint.data"
			t0 := time.Now()

			// timeout the write after 30 sec
			fileSent := make(chan bool)
			var timeOut uint64
			toDur := time.Second * 30
			go func() {
				select {
				case <-fileSent:
				case <-time.After(toDur):
					atomic.AddUint64(&timeOut, 1)
					sessL.Stop()
				}
			}()

			bigfile, err := sessL.SendFile(path, chkptData, time.Now())
			if atomic.LoadUint64(&timeOut) > 0 {
				mylog.Printf("%s timeout after %v on sessL.SendFile() to '%s'", laf.MyID, toDur, cs.follow[i].subj)
				continue
			}
			if err != nil {
				// panic was crashing here with
				// panic: Connect() timeout waiting to SynAck, after
				mylog.Printf("error during lead trying to send "+
					"checkpoint file to '%s': '%v'",
					cs.follow[i].subj, err)
			} else {
				mylog.Printf("lead '%s' sent file '%s' of size %v, stamped '%v', to '%s' in %v", "lead-chkpt-origin;"+cs.lead.subj, bigfile.Filepath, bigfile.SizeInBytes, bigfile.SendTime, cs.follow[i].subj, time.Since(t0))
			}
			sessL.Stop()
		}

	case <-peer.Halt.ReqStop.Chan:
		// shutting down.
		mylog.Printf("shutting down on request from peer.Halt.ReqStop.Chan")
		return ErrShutdown
	}
	return nil
}

func (peer *Peer) amFollow() bool {
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

// StartBackroundRev will keep a peer
// running in the background and
// always accepting and writing checkpoints (as
// long as we are not lead when they are received).
// Track these by their timestamps, and if we have a new one
// (recognized by a more recent timestamp), then
// save it to disk (this dedups if we get multiples of the same).
//
func (peer *Peer) StartBackgroundRecv(myID, myFollowSubj string) {
	mylog.Printf("beginning StartBackgroundRecv(myID='%s', "+
		"myFollowSubj='%s').",
		myID, myFollowSubj)

	go func() {
		defer func() {
			peer.Halt.ReqStop.Close()
			peer.Halt.Done.Close()
			mylog.Printf("StartBackgroundRecv(myID='%s', "+
				"myFollowSubj='%s') has shutdown.",
				myID, myFollowSubj)
		}()

	makeNewCatcher:
		for {
			// accept checkpoints and store them

			// Even if we are lead, run this receive loop
			// in the background so that we are ready if
			// we should become a follower.

			// make a new session without a remote endpoint,
			// so enter Listen state, wait for connection
			// from a lead. Leads won't bother to send to
			// their own recv-chkpoint.* session.
			leadSubj := ""
			sessF, err := swp.SetupRecvStream(
				peer.nc,
				peer.serverOpts.Host,
				peer.serverOpts.Port,
				myID,
				myFollowSubj,
				leadSubj,
				skipTLS,
				nil,
				ignoreSlowConsumerErrors)

			if err != nil {
				mylog.Printf("warning, error back from swp.SetupRecvStream: '%v'", err)
				time.Sleep(time.Second)
				continue
			}

			peer.SetFollowSess(sessF)
			select {
			case <-peer.Halt.ReqStop.Chan:
				//p("%s peer got Halt.ReqStop", myID)
				return
			default:
			}

			var bigfile *swp.BigFile
			//p("%s StartBackgroundRecv: about to RecvFile", myID)

			bigfile, err = sessF.RecvFile() // can hang until sessF is halted.

			//p("%s StartBackgroundRecv: done with RecvFile, err='%v'", myID, err)
			if err != nil {
				if err.Error() != "multiple Read calls return no data or error" {
					if bigfile != nil {
						// typical report is of a zero
						// bigfile that didn't receive
						// anything.
					}
				}
				// are we shutting down?
				select {
				case <-peer.Halt.ReqStop.Chan:
					//p("%s peer got Halt.ReqStop", myID)
					return
				default:
				}
				//p("%s trying to RecvFile again, after 1 sec", myID)
				time.Sleep(time.Second * 2)
				// we close up shop to let outselves pair
				// with any new lead that may come along; otherwise
				// can get stuck on the old one.
				peer.SetFollowSess(nil)
				sessF.Close()
				sessF.Stop()
				continue makeNewCatcher
			}
			// INVAR: err == nil, so our file arrived intact and
			// the blake2b checksum matched. We can and should
			// write this to the saver.

			mylog.Printf("%s follow received good file '%s' of size %v, stamped '%v'.", myID, bigfile.Filepath, bigfile.SizeInBytes, bigfile.SendTime)
			peer.SetFollowSess(nil)
			sessF.Close()
			sessF.Stop()

			err = peer.saver.LocalSet(&KeyInv{Key: []byte("chk"), Val: bigfile.Data, When: bigfile.SendTime})
			if err != nil {
				mylog.Printf("%s peer.saver.WriteKv() got error '%v'", myID, err)
				continue
			}
			mylog.Printf("%s peer.saver.WriteKv() done with nil error, wrote data len = %v.", myID, len(bigfile.Data))

			select {
			case <-peer.Halt.ReqStop.Chan:
				return
			default:
			}
		}
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
