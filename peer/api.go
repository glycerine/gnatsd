package peer

import (
	"time"
)

func (peer *Peer) BcastGetKeyTimes(key []byte) ([]*KeyInv, error) {
	return nil, nil
}

func (peer *Peer) BcastKeyVal(key []byte, data []byte, tm time.Time) error {
	return nil
}

func (peer *Peer) LocalGetKeyVal(key []byte) (data []byte, tm time.Time, err error) {
	return
}

func (peer *Peer) StoreLocalKeyVal(key, val []byte, tm time.Time) error {

	return nil
}

func (peer *Peer) GetLatest(key []byte) (val []byte, tm time.Time, err error) {
	return
}

func (peer *Peer) GetLocal(key []byte) (val []byte, tm time.Time, err error) {
	return
}
