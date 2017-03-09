package swp

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SimNet simulates a network with the given latency and
// loss characteristics. See NewSimNet.
type SimNet struct {
	Net      map[string]chan *Packet
	LossProb float64
	Latency  time.Duration

	TotalSent map[string]int64
	TotalRcvd map[string]int64
	mapMut    sync.Mutex

	// simulate loss of the first packets
	DiscardOnce int64

	// simulate re-ordering of packets by setting this to 1
	SimulateReorderNext int
	heldBack            *Packet

	// simulate duplicating the next packet
	DuplicateNext uint32

	// enforce that advertised windows are never
	// violated by having more messages in flight
	// than have been advertised.
	Advertised map[string]int64
	Inflight   map[string]int64

	ReqStop chan bool
	Done    chan bool

	AllowBlackHoleSends bool
}

// NewSimNet makes a network simulator. The
// latency is one-way trip time; lossProb is the probability of
// the packet getting lost on the network.
func NewSimNet(lossProb float64, latency time.Duration) *SimNet {
	s := &SimNet{
		Net:         make(map[string]chan *Packet),
		LossProb:    lossProb,
		Latency:     latency,
		DiscardOnce: -1,
		TotalSent:   make(map[string]int64),
		TotalRcvd:   make(map[string]int64),
		Advertised:  make(map[string]int64),
		Inflight:    make(map[string]int64),
		ReqStop:     make(chan bool),
		Done:        make(chan bool),
	}
	return s
}

// Listen returns a channel that will be sent on when
// packets have Dest inbox.
func (sim *SimNet) Listen(inbox string) (chan *Packet, error) {
	ch := make(chan *Packet)
	sim.Net[inbox] = ch
	return ch, nil
}

// Send sends the packet pack to pack.Dest. The why
// annoation is optional and allows the logs to illumate
// the purpose of each send (ack, keepAlive, data, etc).
func (sim *SimNet) Send(pack *Packet, why string) error {
	p("in SimNet.Send(pack.SeqNum=%v) why:'%v'", pack.SeqNum, why)

	// try to avoid data races and race detector problems by
	// copying the packet here
	cp := *pack
	pack2 := &cp

	sim.mapMut.Lock()
	sim.TotalSent[pack2.From]++
	defer sim.mapMut.Unlock()

	ch, ok := sim.Net[pack2.Dest]
	if !ok {
		if sim.AllowBlackHoleSends {
			return nil
		}
		return fmt.Errorf("sim sees packet for unknown node '%s'", pack2.Dest)
	}

	switch sim.SimulateReorderNext {
	case 0:
		// do nothing
	case 1:
		sim.heldBack = pack2
		//q("sim reordering: holding back pack SeqNum %v to %v", pack2.SeqNum, pack2.Dest)
		sim.SimulateReorderNext++
		return nil
	default:
		//q("sim: setting SimulateReorderNext %v -> 0", sim.SimulateReorderNext)
		sim.SimulateReorderNext = 0
	}

	if 0 <= pack2.SeqNum && pack2.SeqNum <= sim.DiscardOnce {
		p("sim: packet lost/dropped because %v SeqNum <= DiscardOnce (%v)", pack2.SeqNum, sim.DiscardOnce)
		sim.DiscardOnce = -1
		return nil
	}

	pr := cryptoProb()
	isLost := pr <= sim.LossProb
	if sim.LossProb > 0 && isLost {
		//q("sim: bam! packet-lost! %v to %v", pack2.SeqNum, pack2.Dest)
	} else {
		//q("sim: %v to %v: not lost. packet will arrive after %v", pack2.SeqNum, pack2.Dest, sim.Latency)
		// start a goroutine per packet sent, to simulate arrival time with a timer.
		go sim.sendWithLatency(ch, pack2, sim.Latency)
		if sim.heldBack != nil {
			//q("sim: reordering now -- sending along heldBack packet %v to %v",
			//	sim.heldBack.SeqNum, sim.heldBack.Dest)
			go sim.sendWithLatency(ch, sim.heldBack, sim.Latency+20*time.Millisecond)
			sim.heldBack = nil
		}

		if atomic.CompareAndSwapUint32(&sim.DuplicateNext, 1, 0) {
			go sim.sendWithLatency(ch, pack2, sim.Latency)
		}

	}
	return nil
}

// helper for Send
func (sim *SimNet) sendWithLatency(ch chan *Packet, pack *Packet, lat time.Duration) {
	<-time.After(lat)
	//q("sim: packet %v, after latency %v, ready to deliver to node %v, trying...",
	//	pack.SeqNum, lat, pack.Dest)

	//	sim.preCheckFlowControlNotViolated(pack)

	ch <- pack
	//p("sim: packet (SeqNum: %v) delivered to node %v", pack.SeqNum, pack.Dest)

	//	sim.postCheckFlowControlNotViolated(pack)

	sim.mapMut.Lock()
	sim.TotalRcvd[pack.Dest]++
	sim.mapMut.Unlock()
}

// resolution controls the floating point
// resolution in the cryptoProb routine.
const resolution = 1 << 20

// cryptoProb returns a random number between
// [0, 1] inclusive.
func cryptoProb() float64 {
	b := make([]byte, 8)
	_, err := cryptorand.Read(b)
	panicOn(err)
	r := int(binary.LittleEndian.Uint64(b))
	if r < 0 {
		r = -r
	}
	r = r % (resolution + 1)

	return float64(r) / float64(resolution)
}

// Sum is output by the Simnet.Summary function to
// summarize the packet losses in a network simulation.
type Sum struct {
	ObsKeepRateFromA float64
	ObsKeepRateFromB float64
	tsa              int64
	tra              int64
	tsb              int64
	trb              int64
}

// Summary summarizes the packet drops in a Sum report.
func (net *SimNet) Summary() *Sum {
	net.mapMut.Lock()
	defer net.mapMut.Unlock()

	s := &Sum{
		ObsKeepRateFromA: float64(net.TotalRcvd["B"]) / float64(net.TotalSent["A"]),
		ObsKeepRateFromB: float64(net.TotalRcvd["A"]) / float64(net.TotalSent["B"]),
		tsa:              net.TotalSent["A"],
		tra:              net.TotalRcvd["A"],
		tsb:              net.TotalSent["B"],
		trb:              net.TotalRcvd["B"],
	}
	return s
}

// Print displays the Sum report.
func (s *Sum) Print() {
	fmt.Printf("\n summary: packets A sent %v   -> B packets rcvd %v  [kept %.01f%%, lost %.01f%%]\n",
		s.tsa, s.trb, 100.0*s.ObsKeepRateFromA, 100.0*(1.0-s.ObsKeepRateFromA))
	fmt.Printf("\n summary: packets B sent %v   -> A packets rcvd %v  [kept %.01f%%, lost %.01f%%]\n",
		s.tsb, s.tra, 100.0*s.ObsKeepRateFromB, 100.0*(1.0-s.ObsKeepRateFromB))
}

// HistoryEqual lets one easily compare and send and a recv history
func HistoryEqual(a, b []*Packet) bool {
	na := len(a)
	nb := len(b)
	if na != nb {
		return false
	}
	for i := 0; i < na; i++ {
		if a[i].SeqNum != b[i].SeqNum {
			p("packet histories disagree at i=%v, a[%v].SeqNum = %v, while b[%v].SeqNum = %v",
				i, a[i].SeqNum, b[i].SeqNum)
			return false
		} else {
			p("packet histories both have SeqNum %v", a[i].SeqNum)
		}
	}
	return true
}

func (net *SimNet) Flush() {

}
