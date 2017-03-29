package peer

import (
	"fmt"
	"time"

	"github.com/glycerine/hnatsd/peer/api"
	"github.com/glycerine/hnatsd/peer/gcli"
)

func (peer *Peer) BcastSet(ki *api.KeyInv) error {

	timeout := 120 * time.Second
	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 {
		// no peers, just do the local set.
		//p("no peers in BcastGet!?")
		return peer.LocalSet(ki)
	}
	// save locally
	err = peer.LocalSet(ki)
	if err != nil {
		return err
	}
	numPeers := len(peers.Members)
	//p("BcastSet sees numPeers = %v", numPeers)

	cs, _ := list2status(peers)
	mylog.Printf("BcastSet: we have clusterStatus: '%s'", &cs)

	req := &api.BcastSetRequest{
		Ki: ki,
	}

	return peer.doGrpcClientSendFileSetRequest(req, &cs)
}

func (peer *Peer) BcastGet(key []byte, includeValue bool, timeout time.Duration, who string) (kis []*api.KeyInv, err error) {

	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 {
		// no peers, just do the local get.
		//p("no peers in BcastGet!?")
		ki, err := peer.LocalGet(key, includeValue)
		if err != nil {
			return nil, err
		}
		kis = append(kis, ki)
		return kis, err
	}
	numPeers := len(peers.Members)
	if who != "" {
		// request to restrict to just one peer.
		numPeers = 1
	}
	//p("numPeers = %v", numPeers)

	bgr := &api.BcastGetRequest{
		Key:            key,
		Who:            who,
		IncludeValue:   includeValue,
		ReplyGrpcHost:  peer.GservCfg.Host,
		ReplyGrpcXPort: peer.GservCfg.ExternalLsnPort,
		ReplyGrpcIPort: peer.GservCfg.InternalLsnPort,
	}
	mm, err := bgr.MarshalMsg(nil)
	if err != nil {
		return nil, err
	}

	err = peer.nc.Publish(peer.subjBcastGet, mm)
	if err != nil {
		return nil, err
	}
	toCh := time.After(timeout)

	sorter := NewInventory()
	for i := 0; i < numPeers; i++ {
		select {
		case <-toCh:
			return nil, ErrTimedOut
		case reply := <-peer.GservCfg.ServerGotReply:
			p("BcastGet got a reply, on i = %v", i)
			if bgr.Err != "" {
				return nil, fmt.Errorf(bgr.Err)
			} else {
				bgr.Ki.When = bgr.Ki.When.UTC()
				sorter.Upsert(bgr.Ki)

				// save... if it is a BcastSet
				ki := reply.Ki
				if len(ki.Val) > 0 {
					err = s.peer.LocalSet(
						&api.KeyInv{Key: ki.Key, Val: ki.Val, When: ki.When},
					)
					p("server sees the last chunk of '%s', writing to bolt key='%s' len: %v bytes gave '%v', and returning now.", nk.Filepath, string(ki.Key), len(ki.Val), err)
					p("debug! server saw ki.Val='%s'", string(ki.Val))
					if err != nil {
						return fmt.Errorf("gserv/server.go SendFile(): s.peer.LocalSet() errored '%v'", err)
					}
				}

			}
		}
	}
	p("done with collection loop, we have %v replies", sorter.Len())
	// sorter sorts them by key, then time, then who.
	for it := sorter.Min(); !it.Limit(); it = it.Next() {
		kis = append(kis, it.Item().(*api.KeyInv))
	}
	return
}

func (peer *Peer) LocalGet(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	return peer.saver.LocalGet(key, includeValue)
}

func (peer *Peer) LocalSet(ki *api.KeyInv) error {
	return peer.saver.LocalSet(ki)
}

func (peer *Peer) GetLatest(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	kis, err := peer.BcastGet(key, false, time.Second*60, "")
	if err != nil {
		return nil, err
	}
	//p("kis=%#v", kis)
	// come back sorted by time, so latest is the last.
	n := len(kis)
	target := kis[n-1]
	if !includeValue {
		return target, nil
	}
	// now fetch the data
	kisData, err := peer.BcastGet(key, true, time.Second*60, target.Who)
	if err != nil {
		return nil, err
	}
	return kisData[0], nil
}

const _EMPTY_ = ""
const RequestChanLen = 8

var ErrTimedOut = fmt.Errorf("timed out")

func (peer *Peer) doGrpcClientSendFileSetRequest(req *api.BcastSetRequest, cs *clusterStatus) error {
	reqBytes, err := req.MarshalMsg(nil)
	if err != nil {
		return err
	}

	isBcastSet := true

	for _, follower := range cs.follow {
		host := follower.loc.Host
		port := follower.loc.GrpcPort
		iport := follower.loc.InternalPort

		clicfg := &gcli.ClientConfig{
			AllowNewServer:          peer.SshClientAllowsNewSshdServer,
			TestAllowOneshotConnect: peer.TestAllowOneshotConnect,
			ServerHost:              host,
			ServerPort:              port,
			ServerInternalHost:      "127.0.0.1",
			ServerInternalPort:      iport,

			Username:             peer.SshClientLoginUsername,
			PrivateKeyPath:       peer.SshClientPrivateKeyPath,
			ClientKnownHostsPath: peer.SshClientClientKnownHostsPath,
		}

		replyData, err := reply.MarshalMsg(nil)
		panicOn(err)

		err = clicfg.ClientSendFile(string(req.Ki.Key), reqBytes, isBcastSet)
		panicOn(err)
		p("BcastSet successfully clicfg.ClientSendFile to %s:%v", host, port)
	}

}

func (peer *Peer) clientDoGrpcSendFileBcastGetReply(bgr *api.BcastGetRequest, reply *api.BcastGetReply) error {
	clicfg := &gcli.ClientConfig{
		AllowNewServer:          peer.SshClientAllowsNewSshdServer,
		TestAllowOneshotConnect: peer.TestAllowOneshotConnect,
		ServerHost:              bgr.ReplyGrpcHost,
		ServerPort:              bgr.ReplyGrpcXPort,
		ServerInternalHost:      "127.0.0.1",
		ServerInternalPort:      bgr.ReplyGrpcIPort,

		Username:             peer.SshClientLoginUsername,
		PrivateKeyPath:       peer.SshClientPrivateKeyPath,
		ClientKnownHostsPath: peer.SshClientClientKnownHostsPath,
	}

	replyData, err := reply.MarshalMsg(nil)
	panicOn(err)

	isBcastSet := false
	return clicfg.ClientSendFile(string(bgr.Key), replyData, isBcastSet)
}
