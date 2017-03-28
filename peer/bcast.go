package peer

import (
	"fmt"
	"time"

	"github.com/glycerine/nats"
)

func (peer *Peer) BcastSet(ki *KeyInv) error {

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

	req := &BcastSetRequest{
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
			var rep BcastSetReply
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
