package peer

import (
	"time"
)

// KeyInv supplies the keys and their
// peer location (Who) and their timestamps
// (When) without (necessarily) providing
// their data Val.
type KeyInv struct {
	Key  []byte
	Who  string
	When time.Time
	Val  []byte
}

func (peer *Peer) BcastGet(key []byte, includeValue bool) (kis []*KeyInv, err error) {
	ret := []*KeyInv{}
	return ret, nil
}

func (peer *Peer) LocalGet(key []byte, includeValue bool) (ki *KeyInv, err error) {
	ret := &KeyInv{}
	return ret, nil
}

func (peer *Peer) BcastSet(ki *KeyInv) error {
	return nil
}

func (peer *Peer) LocalSet(ki *KeyInv) error {
	return nil
}

func (peer *Peer) GetLatest(key []byte, includeValue bool) (ki *KeyInv, err error) {
	ret := &KeyInv{}
	return ret, nil
}
