// package bchan provides 1:M value-broadcasting channels.
//
// Receivers from bchan broadcast channels must be aware that they
// are using a bchan.Ch channel and call BcastAck() after
// every receive. Failure to do this will result in other
// receivers blocking instead of receiving the broadcast value.
//
package bchan

import (
	"sync"
)

// Bchan is an 1:M non-blocking value-loadable channel.
// The client needs to only know about one
// rule: after a receive on Ch, you must call Bchan.BcastAck().
//
type Bchan struct {
	Ch  chan interface{}
	mu  sync.Mutex
	on  bool
	cur interface{}
}

// New constructor should be told
// how many recipients are expected in
// expectedDiameter. If the expectedDiameter
// is wrong the Bchan will still function,
// but you may get slower concurrency
// than if the number is accurate. It
// is fine to overestimate the diameter by
// a little or even be off completely,
// but the extra slots in the buffered channel
// take up some memory and will add
// linearly to the service time
// as they are maintained.
func New(expectedDiameter int) *Bchan {
	if expectedDiameter <= 0 {
		expectedDiameter = 1
	}
	return &Bchan{
		Ch: make(chan interface{}, expectedDiameter+1),
	}
}

// On turns on the broadcast channel without
// changing the value to be transmitted.
//
func (b *Bchan) On() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.on = true
	b.fill()
}

// Set stores a value to be broadcast
// and clears any prior queued up
// old values. Call On() after set
// to activate the new value.
// See also Bcast() that does Set()
// followed by On() in one call.
//
func (b *Bchan) Set(val interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cur = val
	b.drain()
}

// Get returns the currently set
// broadcast value.
func (b *Bchan) Get() interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cur
}

// Bcast is the common case of doing
// both Set() and then On() together
// to start broadcasting a new value.
//
func (b *Bchan) Bcast(val interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cur = val
	b.drain()
	b.on = true
	b.fill()
}

// Clear turns off broadcasting and
// empties the channel of any old values.
func (b *Bchan) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.on = false
	b.drain()
	b.cur = nil
}

// drain all messages, leaving b.Ch empty.
// Users typically want Clear() instead.
func (b *Bchan) drain() {
	// empty chan
	for {
		select {
		case <-b.Ch:
		default:
			return
		}
	}
}

// BcastAck must be called immediately after
// a client receives on Ch. All
// clients on every channel receive must call BcastAck after receiving
// on the channel Ch. This makes such channels
// self-servicing, as BcastAck will re-fill the
// async channel with the current value.
func (b *Bchan) BcastAck() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.on {
		b.fill()
	}
}

// fill up the channel
func (b *Bchan) fill() {
	for {
		select {
		case b.Ch <- b.cur:
		default:
			return
		}
	}
}
