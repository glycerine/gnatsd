package peer

import (
	"time"
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

func (peer *Peer) BcastGet(key []byte, includeValue bool) (kis []*KeyInv, err error) {
	ret := []*KeyInv{}
	return ret, nil
}

func (peer *Peer) LocalGet(key []byte, includeValue bool) (ki *KeyInv, err error) {
	return peer.saver.LocalGet(key, includeValue)
}

func (peer *Peer) BcastSet(ki *KeyInv) error {
	return nil
}

func (peer *Peer) LocalSet(ki *KeyInv) error {
	return peer.saver.LocalSet(ki)
}

func (peer *Peer) GetLatest(key []byte, includeValue bool) (ki *KeyInv, err error) {
	ret := &KeyInv{}
	return ret, nil
}
