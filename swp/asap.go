package swp

import (
	"sync"
)

// AsapHelper is a simple queue
// goroutine that delivers packets
// to ASAP clients as soon as they
// become avaialable. Packets may
// be dropped, duplicated, or
// misordered, but they will be
// delivered as soon as possible.
type AsapHelper struct {
	ReqStop chan bool
	Done    chan bool

	// drop packets at this size limite,
	// but discard the first rather than
	// the last, so new info can be seen
	// rather than stale.
	Limit int64

	rcv     chan *Packet
	enqueue chan *Packet
	mut     sync.Mutex
	q       []*Packet
}

// NewAsapHelper creates a new AsapHelper.
// Callers provide rcvUnordered which which they
// should then do blocking receives on to
// aquire new *Packets out of order but As
// Soon As Possible.
func NewAsapHelper(rcvUnordered chan *Packet, max int64) *AsapHelper {
	return &AsapHelper{
		ReqStop: make(chan bool),
		Done:    make(chan bool),
		rcv:     rcvUnordered,
		enqueue: make(chan *Packet),
		Limit:   max,
	}
}

// Stop shuts down the AsapHelper goroutine.
func (r *AsapHelper) Stop() {
	r.mut.Lock()
	select {
	case <-r.ReqStop:
	default:
		close(r.ReqStop)
	}
	r.mut.Unlock()
	<-r.Done
}

// Start starts the AsapHelper tiny queuing service.
func (r *AsapHelper) Start() {
	go func() {
		var rch chan *Packet
		var next *Packet
		for {
			if next == nil {
				if len(r.q) > 0 {
					next = r.q[0]
					r.q = r.q[1:]
					rch = r.rcv
				} else {
					rch = nil
				}
			} else {
				rch = r.rcv
			}

			select {
			case rch <- next:
				next = nil
			case pack := <-r.enqueue:
				r.q = append(r.q, pack)
				if int64(len(r.q)) > r.Limit {
					r.q = r.q[1:]
				}
			case <-r.ReqStop:
				close(r.Done)
				return
			}
		}
	}()
}
