package peer

import (
	"fmt"
	"time"

	"github.com/glycerine/hnatsd/peer/api"

	"github.com/glycerine/nats"
)

func (peer *Peer) BcastSet(ki *api.KeyInv) error {

	timeout := 120 * time.Second
	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 {
		// no peers, just do the local get.
		//p("no peers in BcastGet!?")
		return peer.LocalSet(ki)
	}
	numPeers := len(peers.Members)
	//p("BcastSet sees numPeers = %v", numPeers)

	cs, _ := list2status(peers)
	mylog.Printf("BcastSet: we have clusterStatus: '%s'", &cs)

	req := &api.BcastSetRequest{
		Ki: ki,
	}
	mm, err := req.MarshalMsg(nil)
	if err != nil {
		return err
	}

	inbox := nats.NewInbox()
	ch := make(chan *nats.Msg, RequestChanLen)

	s, err := peer.nc.ChanSubscribe(inbox, ch)
	if err != nil {
		return err
	}
	s.SetPendingLimits(-1, -1)
	defer s.Unsubscribe()

	err = peer.nc.PublishRequest(peer.subjBcastSet, inbox, mm)
	if err != nil {
		return err
	}
	toCh := time.After(timeout)

	errs := ""
	for i := 0; i < numPeers; i++ {
		select {
		case <-toCh:
			return ErrTimedOut
		case reply := <-ch:
			//p("BcastSet got a reply, on i = %v", i)
			var rep api.BcastSetReply
			_, err := rep.UnmarshalMsg(reply.Data)
			if err != nil {
				errs += err.Error() + ";"
			} else {
				if rep.Err != "" {
					errs += rep.Err + ";"
				}
			}
		}
	}
	if errs == "" {
		return nil
	}
	return fmt.Errorf(errs)
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
		case bgr := <-peer.GservCfg.ServerGotReply:
			//p("BcastGet got a reply, on i = %v", i)
			if bgr.Err != "" {
				return nil, fmt.Errorf(bgr.Err)
			} else {
				bgr.Ki.When = bgr.Ki.When.UTC()
				sorter.Upsert(bgr.Ki)
			}
		}
	}
	//p("done with collection loop, we have %v replies", sorter.Len())
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
