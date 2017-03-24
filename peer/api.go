package peer

import (
	"fmt"
	"time"

	"github.com/glycerine/nats"
)

//go:generate msgp

// KeyInv supplies the keys and their
// peer location (Who) and their timestamps
// (When) while optionally (but not necessarily)
// providing their data Val.
//
// The includeValue flag in the
// calls below determines if we return the Val
// on Get calls. Val must always be provided
// on Set.
//
type KeyInv struct {
	Key  []byte
	Who  string
	When time.Time
	Size int64
	Val  []byte
}

type BcastGetRequest struct {

	// Key specifies the key to query and return the value of.
	Key []byte

	// Who should be left empty to get all replies.
	// Otherwise only the peer whose name matches will reply.
	Who string

	// IncludeValue when false returns the timestamp and size without
	// the whole (big) value.
	IncludeValue bool
}

type BcastGetReply struct {
	Ki  *KeyInv
	Err string
}

type BcastSetRequest struct {
	Ki *KeyInv
}

type BcastSetReply struct {
	Err string
}

const _EMPTY_ = ""
const RequestChanLen = 8

var ErrTimedOut = fmt.Errorf("timed out")

func (peer *Peer) BcastGet(key []byte, includeValue bool, timeout time.Duration, who string) (kis []*KeyInv, err error) {

	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 {
		// no peers, just do the local get.
		p("no peers in BcastGet!?")
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
	p("numPeers = %v", numPeers)

	bgr := &BcastGetRequest{
		Key:          key,
		Who:          who,
		IncludeValue: includeValue,
	}
	mm, err := bgr.MarshalMsg(nil)
	if err != nil {
		return nil, err
	}

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
	toCh := time.After(timeout)

	sorter := NewInventory()
	for i := 0; i < numPeers; i++ {
		select {
		case <-toCh:
			return nil, ErrTimedOut
		case reply := <-ch:
			p("BcastGet got a reply, on i = %v", i)
			var bgr BcastGetReply
			_, err := bgr.UnmarshalMsg(reply.Data)
			if err != nil {
				return nil, err
			}
			if bgr.Err != "" {
				return nil, fmt.Errorf(bgr.Err)
			} else {
				bgr.Ki.When = bgr.Ki.When.UTC()
				sorter.Upsert(bgr.Ki)
			}
		}
	}
	p("done with collection loop, we have %v replies", sorter.Len())
	// sorter sorts them by key, then time, then who.
	for it := sorter.Min(); !it.Limit(); it = it.Next() {
		kis = append(kis, it.Item().(*KeyInv))
	}
	return
}
func (peer *Peer) BcastSet(ki *KeyInv) error {

	timeout := 10 * time.Second
	peers, err := peer.GetPeerList(timeout)
	if err != nil || peers == nil || len(peers.Members) <= 1 {
		// no peers, just do the local get.
		p("no peers in BcastGet!?")
		return peer.LocalSet(ki)
	}
	numPeers := len(peers.Members)
	p("BcastSet sees numPeers = %v", numPeers)

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
			p("BcastSet got a reply, on i = %v", i)
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

func (peer *Peer) LocalGet(key []byte, includeValue bool) (ki *KeyInv, err error) {
	return peer.saver.LocalGet(key, includeValue)
}

func (peer *Peer) LocalSet(ki *KeyInv) error {
	return peer.saver.LocalSet(ki)
}

func (peer *Peer) GetLatest(key []byte, includeValue bool) (ki *KeyInv, err error) {
	kis, err := peer.BcastGet(key, false, time.Second*60, "")
	if err != nil {
		return nil, err
	}
	p("kis=%#v", kis)
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
