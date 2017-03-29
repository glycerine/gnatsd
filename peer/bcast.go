package peer

import (
	"fmt"
	"log"
	"time"

	"github.com/glycerine/hnatsd/peer/api"
	"github.com/glycerine/hnatsd/peer/gcli"
	"github.com/glycerine/nats"
)

func (peer *Peer) ClientInitiateBcastGet(key []byte, includeValue bool, timeout time.Duration, who string) (kis []*api.KeyInv, err error) {

	//p("%s top of ClientInitiateBcastGet(). who='%s'.", peer.loc.ID, who)

	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 || who == peer.loc.ID {
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

	// actually make the request over NATS
	bgr := &api.BcastGetRequest{
		FromID:         peers.MyID,
		Key:            key,
		Who:            who,
		IncludeValue:   includeValue,
		ReplyGrpcHost:  peer.GservCfg.Host,
		ReplyGrpcXPort: peer.GservCfg.ExternalLsnPort,
		ReplyGrpcIPort: peer.GservCfg.InternalLsnPort,
	}
	mm, err := bgr.MarshalMsg(nil)
	panicOn(err)

	// slightly different sending over NATS, depending
	// on how the reply is expected.

	sorter := NewInventory()
	if includeValue {
		// gRPC response expected

		err = peer.nc.Publish(peer.subjBcastGet, mm)
		if err != nil {
			return nil, err
		}
		peer.nc.Flush()

		toCh := time.After(timeout)

		for i := 0; i < numPeers; i++ {
			select {
			case <-toCh:
				return nil, ErrTimedOut

			case <-peer.Halt.ReqStop.Chan:
				peer.Halt.Done.Close()
				return nil, ErrShutdown

			case bgReply := <-peer.GservCfg.ServerGotGetReply:
				//p("BcastGet got a reply, on i = %v", i)
				if bgReply.Err != "" {
					return nil, fmt.Errorf(bgReply.Err)
				} else {
					bgReply.Ki.When = bgReply.Ki.When.UTC()
					sorter.Upsert(bgReply.Ki)
				}
			}
		}
	} else {
		// NATS response expected

		inbox := nats.NewInbox()
		ch := make(chan *nats.Msg, RequestChanLen)
		s, err := peer.nc.ChanSubscribe(inbox, ch)
		if err != nil {
			return nil, err
		}
		defer s.Unsubscribe()

		err = peer.nc.PublishRequest(peer.subjBcastGet, inbox, mm)
		if err != nil {
			return nil, err
		}
		peer.nc.Flush()
		toCh := time.After(timeout)

		for i := 0; i < numPeers; i++ {
			select {
			case <-toCh:
				return nil, ErrTimedOut
			case reply := <-ch:
				//p("BcastGet got a reply, on i = %v. len(reply.Data)=%v", i, len(reply.Data))
				if len(reply.Data) == 0 {
					panic("reply with no data")
				}
				var bgReply api.BcastGetReply
				_, err := bgReply.UnmarshalMsg(reply.Data)
				if err != nil {
					return nil, err
				}
				//p("bgReply = %#v", bgReply)
				if bgReply.Err != "" {
					return nil, fmt.Errorf(bgReply.Err)
				} else {
					bgReply.Ki.When = bgReply.Ki.When.UTC()
					sorter.Upsert(bgReply.Ki)
				}
			}
		}
	} // end else NATS response

	//p("done with collection loop, we have %v replies", sorter.Len())
	// sorter sorts them by key, then time, then who.
	for it := sorter.Min(); !it.Limit(); it = it.Next() {
		kis = append(kis, it.Item().(*api.KeyInv))
	}
	return kis, nil
}

// pull the local boltdb version of the key and
// send it back. User nats if only the key's metadata
// was requested. Use gRPC for sending the large file.
//
func (peer *Peer) ServerHandleBcastGet(msg *nats.Msg) error {

	var bgr api.BcastGetRequest
	bgr.UnmarshalMsg(msg.Data)
	mylog.Printf("%s peer recevied subjBcastGet for key '%s'",
		peer.saver.whoami, string(bgr.Key))

	key := bgr.Key
	who := bgr.Who
	includeValue := bgr.IncludeValue

	//p("%s top of ServerHandleBcastGet(). who='%s'.", peer.loc.ID, who)

	// it may well be that peer.loc.ID != peer.saver.whoami

	// are we filtered down to a specific peer request?
	if who != "" {
		// yep
		if who == peer.loc.ID || who == peer.saver.whoami {
			//p("%s sees peer-specific BcastGet request! matched either peer.loc.ID='%s' or peer.saver.whoami='%s'", who, peer.loc.ID, peer.saver.whoami)

		} else {
			//p("%s / ID:%s sees peer-specific BcastGet request for '%s' which is not us!", peer.saver.whoami, peer.loc.ID, who)
			return nil
		}
	} else {
		//p("Who was not set...")
	}

	// assemble our response/reply
	var reply api.BcastGetReply

	ki, err := peer.LocalGet(key, includeValue)
	if err != nil {
		mylog.Printf("peer.LocalGet('%s' returned error '%v'", string(key), err)
		reply.Err = err.Error()
	} else {
		reply.Ki = ki
	}
	//p("debug peer.LocalGet(key='%s') returned ki.Key='%s' and ki.Val='%s'", string(key), string(ki.Key), string(ki.Val[:intMin(100, len(ki.Val))]))

	// Asking for big data or no?
	// For big data transfer, use gRPC. Can handle unlimited size.
	// For small meta-data request, use NATS. Will be faster. Small messages only.
	if includeValue {
		// gRPC response

		// use the SendFile() client to return the BigFile
		err = peer.clientDoGrpcSendFileBcastGetReply(&bgr, &reply)
		panicOn(err)

	} else {
		// NATS response

		mm, err := reply.MarshalMsg(nil)
		panicOn(err)
		err = peer.nc.Publish(msg.Reply, mm)
		panicOn(err)
	}
	return nil
}

func (peer *Peer) LocalGet(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	return peer.saver.LocalGet(key, includeValue)
}

func (peer *Peer) LocalSet(ki *api.KeyInv) error {
	return peer.saver.LocalSet(ki)
}

func (peer *Peer) GetLatest(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	kis, err := peer.ClientInitiateBcastGet(key, false, time.Second*60, "")
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
	kisData, err := peer.ClientInitiateBcastGet(key, true, time.Second*60, target.Who)
	if err != nil {
		return nil, err
	}
	return kisData[0], nil
}

func (peer *Peer) BcastSet(ki *api.KeyInv) error {
	//p("%s top of BcastSet", peer.loc.ID)

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
	//numPeers := len(peers.Members)
	//p("BcastSet sees numPeers = %v", numPeers)

	cs, _ := list2status(peers)
	mylog.Printf("BcastSet: we have clusterStatus: '%s'", &cs)

	req := &api.BcastSetRequest{
		Ki:     ki,
		FromID: peers.MyID,
	}

	return peer.doGrpcClientSendFileSetRequest(req, &cs)
}

const _EMPTY_ = ""
const RequestChanLen = 8

var ErrTimedOut = fmt.Errorf("timed out")

func (peer *Peer) doGrpcClientSendFileSetRequest(req *api.BcastSetRequest, cs *clusterStatus) error {
	//p("%s top of BCAST SET doGrpcClientSendFileSetRequest()", peer.loc.ID)

	reqBytes, err := req.MarshalMsg(nil)
	if err != nil {
		return err
	}

	isBcastSet := true

	for _, follower := range cs.follow {
		//p("%s BCAST SET doGrpcClientSendFileSetRequest(): on follower %v of %v (follower.loc='%s')", peer.loc.ID, k, len(cs.follow), follower.loc.ID)

		// need full location with Interal + Grpc ports
		peer.mut.Lock()
		fullLoc, ok := peer.lastSeenInternalPortAloc[follower.loc.ID]
		peer.mut.Unlock()
		if !ok {
			log.Printf("no full location available for follower.loc.ID='%s'",
				follower.loc.ID)
			continue
		} else {
			//p("fullLoc=%#v", fullLoc)
		}

		host := fullLoc.Host
		eport := fullLoc.Grpc.ExternalPort
		iport := fullLoc.Grpc.InternalPort

		clicfg := &gcli.ClientConfig{
			AllowNewServer:          peer.SshClientAllowsNewSshdServer,
			TestAllowOneshotConnect: peer.TestAllowOneshotConnect,
			ServerHost:              host,
			ServerPort:              eport,
			ServerInternalHost:      "127.0.0.1",
			ServerInternalPort:      iport,

			Username:             peer.SshClientLoginUsername,
			PrivateKeyPath:       peer.SshClientPrivateKeyPath,
			ClientKnownHostsPath: peer.SshClientClientKnownHostsPath,
		}

		clicfg.ClientSendFile(string(req.Ki.Key), reqBytes, isBcastSet)

		//p("%s in doGrpcClientSendFileSetRequest, after clicfg.ClientSendFile()", peer.loc.ID)
	}

	//p("%s BcastSet successfully doGrpcClientSendFileSetRequest to %s:%v", peer.loc.ID, host, eport)

	return nil
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

func (peer *Peer) BackgroundReceiveBcastSetAndWriteToBolt() {
	//p("%s top of BackgroundReceiveBcastSetAndWriteToBolt", peer.loc.ID)
	go func() {
		for {
			//			if peer.GservCfg == nil || peer.GservCfg.ServerGotSetRequest == nil {
			//				time.Sleep(time.Second)
			//				continue
			//			}
			select {
			case setReq := <-peer.GservCfg.ServerGotSetRequest:
				//p("%s got setReq of len %v from %s on channel peer.GservCfg.ServerGotSetRequest", peer.loc.ID, setReq.FromID, len(setReq.Ki.Val))

				// write to bolt
				ki := setReq.Ki
				if len(ki.Val) > 0 {
					err := peer.LocalSet(ki)
					//p("debug! bcast.go wrote ki.Val='%s'", string(ki.Val[:intMin(100, len(ki.Val))]))
					if err != nil {
						log.Printf("gserv/server.go SendFile(): s.peer.LocalSet() errored '%v'", err)
						continue
					}
				} else {
					panic("strange: got setReq with zero payload!")
				}

			case <-peer.Halt.ReqStop.Chan:
				//p("BackgroundReceiveBcastSetAndWriteToBolt exiting on Halt.ReqStop")
				peer.Halt.Done.Close()
				return
			}
		}
	}()
}

/*
	bgr := &api.BcastGetRequest{
		FromID:         peers.MyID,
		Key:            key,
		Who:            who,
		IncludeValue:   includeValue,
		ReplyGrpcHost:  peer.GservCfg.Host,
		ReplyGrpcXPort: peer.GservCfg.ExternalLsnPort,
		ReplyGrpcIPort: peer.GservCfg.InternalLsnPort,
	}
*/
//	p("BcastGet handler about to Publish on msg.Reply='%s'", msg.Reply)
