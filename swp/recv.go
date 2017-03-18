package swp

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b
	"github.com/glycerine/idem"
)

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Received bool
	Pack     *Packet
}

// RecvState tracks the receiver's sliding window state.
type RecvState struct {
	Clk                     Clock
	Net                     Network
	Inbox                   string
	NextFrameExpected       int64
	LastFrameClientConsumed int64
	Rxq                     []*RxqSlot
	RecvWindowSize          int64
	RecvWindowSizeBytes     int64
	mut                     sync.Mutex
	Timeout                 time.Duration
	RecvHistory             []*Packet

	MsgRecv chan *Packet

	Halt *idem.Halter
	//	ReqStop chan bool
	//	Done    chan bool

	RecvSz       int64
	DiscardCount int64

	snd *SenderState

	LastMsgConsumed    int64
	LargestSeqnoRcvd   int64
	MaxCumulBytesTrans int64
	LastByteConsumed   int64

	LastAvailReaderBytesCap int64
	LastAvailReaderMsgCap   int64

	RcvdButNotConsumed map[int64]*Packet

	ReadyForDelivery []*Packet
	ReadMessagesCh   chan InOrderSeq
	NumHeldMessages  chan int64

	// If AsapOn is true the recevier will
	// forward packets for delivery to a
	// client as soon as they arrive
	// but without ordering guarantees;
	// and we may also drop packets if
	// the receive doesn't happen within
	// 100 msec.
	//
	// The client must have previously called
	// Session.RegisterAsap and provided a
	// channel to receive *Packet on.
	//
	// As-soon-as-possible delivery has no
	// effect on the flow-control properties
	// of the session, nor on the delivery
	// to the one-time/order-preserved clients.
	AsapOn        bool
	asapHelper    *AsapHelper
	setAsapHelper chan *AsapHelper
	testing       *testCfg

	TcpState TcpState

	AppCloseCallback  func()
	AcceptReadRequest chan *ReadRequest
}

// InOrderSeq represents ordered (and gapless)
// data as delivered to the consumer application.
// The appliation requests
// it by asking on the Session.ReadMessagesCh channel.
type InOrderSeq struct {
	Seq []*Packet
}

// NewRecvState makes a new RecvState manager.
func NewRecvState(net Network, recvSz int64, recvSzBytes int64, timeout time.Duration,
	inbox string, snd *SenderState, clk Clock) *RecvState {

	r := &RecvState{
		Clk:                 clk,
		Net:                 net,
		Inbox:               inbox,
		RecvWindowSize:      recvSz,
		RecvWindowSizeBytes: recvSzBytes,
		Rxq:                 make([]*RxqSlot, recvSz),
		Timeout:             timeout,
		RecvHistory:         make([]*Packet, 0),
		Halt:                idem.NewHalter(),
		RecvSz:              recvSz,
		snd:                 snd,
		RcvdButNotConsumed:  make(map[int64]*Packet),
		ReadyForDelivery:    make([]*Packet, 0),
		ReadMessagesCh:      make(chan InOrderSeq),
		LastMsgConsumed:     -1,
		LargestSeqnoRcvd:    -1,
		MaxCumulBytesTrans:  0,
		LastByteConsumed:    -1,
		NumHeldMessages:     make(chan int64),
		setAsapHelper:       make(chan *AsapHelper),
		TcpState:            Established,
		AcceptReadRequest:   make(chan *ReadRequest),
	}

	for i := range r.Rxq {
		r.Rxq[i] = &RxqSlot{}
	}
	return r
}

// Start begins receiving. RecvStates receives both
// data and acks from earlier sends.
// Start launches a go routine in the background.
func (r *RecvState) Start() error {
	mr, err := r.Net.Listen(r.Inbox)
	if err != nil {
		return err
	}
	switch nn := r.Net.(type) {
	case *NatsNet:
		//p("%v receiver setting nats subscription buffer limits", r.Inbox)
		// NB: we have to allow somewhat *more* than than data
		// limits to allow nats to deliver control messages such
		// as acks and keep-alives.
		flow := r.snd.FlowCt.GetFlow()
		SetSubscriptionLimits(nn.Cli.Scrip,
			r.RecvWindowSize+flow.ReservedMsgCap,
			r.RecvWindowSizeBytes+flow.ReservedByteCap)
	}
	r.MsgRecv = mr

	var deliverToConsumer chan InOrderSeq
	var delivery InOrderSeq

	go func() {
		defer func() {
			///p("%s RecvState defer/shutdown happening.", r.Inbox)
			// are we closing too fast?
			r.Halt.RequestStop()
			r.Halt.MarkDone()
			r.cleanupOnExit()
			if r.snd != nil && r.snd.Halt != nil {
				r.snd.Halt.RequestStop()
			}
		}()

	recvloop:
		for {
			//p("%v top of recvloop, receiver NFE: %v",
			//	r.Inbox, r.NextFrameExpected)

			deliverToConsumer = nil
			if len(r.ReadyForDelivery) > 0 {
				delivery.Seq = r.ReadyForDelivery
				deliverToConsumer = r.ReadMessagesCh

				//deliveryLen := len(delivery.Seq)
				//p("%v recloop has len %v r.ReadyForDelivery, SeqNum from [%v, %v]", r.Inbox, len(r.ReadyForDelivery), delivery.Seq[0].SeqNum, delivery.Seq[deliveryLen-1].SeqNum)
			}

			//p("recvloop: about to select")
			select {
			case helper := <-r.setAsapHelper:
				//p("recvloop: got <-r.setAsapHelper")
				// stop any old helper
				if r.asapHelper != nil {
					r.asapHelper.Stop()
				}
				r.asapHelper = helper
				if helper != nil {
					r.AsapOn = true
				}

			case r.NumHeldMessages <- int64(len(r.RcvdButNotConsumed)):
				//p("recvloop: got <-r.RcvdButNotConsumed")

			case rr := <-r.AcceptReadRequest:
				if len(delivery.Seq) == 0 {
					// nothing to deliver
					close(rr.Done)
					continue
				}
				// something to deliver
				r.fillAsMuchAsPossible(rr)
				close(rr.Done)
				continue

			case deliverToConsumer <- delivery:
				deliveryLen := len(delivery.Seq)
				//p("%v made deliverToConsumer delivery of %v packets [%v, %v]", r.Inbox, deliveryLen, delivery.Seq[0].SeqNum, delivery.Seq[deliveryLen-1].SeqNum)
				for _, pack := range delivery.Seq {
					///p("%v after delivery, deleting from r.RcvdButNotConsumed pack.SeqNum=%v", r.Inbox, pack.SeqNum)
					delete(r.RcvdButNotConsumed, pack.SeqNum)
					r.LastMsgConsumed = pack.SeqNum
				}
				// this seems wrong:
				//r.LastByteConsumed = delivery.Seq[0].CumulBytesTransmitted - int64(len(delivery.Seq[0].Data))
				// this seems right:
				r.LastByteConsumed = delivery.Seq[deliveryLen-1].CumulBytesTransmitted

				r.ReadyForDelivery = make([]*Packet, 0)
				lastPack := delivery.Seq[deliveryLen-1]
				r.LastFrameClientConsumed = lastPack.SeqNum
				r.ack(r.LastFrameClientConsumed, lastPack, EventDataAck)
				delivery.Seq = nil

			case <-r.Halt.ReqStop.Chan:
				///p("%v recvloop sees ReqStop, shutting down.", r.Inbox)
				return
			case <-r.snd.SenderShutdown:
				///p("recvloop: got <-r.snd.SenderShutdown. note that len(r.RcvdButNotConsumed)=%v", len(r.RcvdButNotConsumed))
				return
			case pack := <-r.MsgRecv:
				///p("%v recvloop sees packet.SeqNum '%v', event:'%s', AckNum:%v", r.Inbox, pack.SeqNum, pack.TcpEvent, pack.AckNum)
				// test instrumentation, used e.g. in clock_test.go
				if r.testing != nil && r.testing.incrementClockOnReceive {
					r.Clk.(*SimClock).Advance(time.Second)
				}

				if len(pack.Data) > 0 {
					chk := Blake2bOfBytes(pack.Data)
					if 0 != bytes.Compare(pack.Blake2bChecksum, chk) {
						p("expected checksum to be '%x', but was '%x'. For pack.SeqNum %v",
							pack.Blake2bChecksum, chk, pack.SeqNum)
						panic("data corruption detected by blake2b checksum")

						// if we aren't going to panic, then at least drop the packet.
						continue recvloop
					} else {
						//p("good: checksums match")
					}
				}

				now := r.Clk.Now()
				pack.ArrivedAtDestTm = now

				if r.testing != nil && r.testing.ackCb != nil {
					r.testing.ackCb(pack)
				}

				// tell any ASAP clients about it
				if r.AsapOn && r.asapHelper != nil {
					select {
					case r.asapHelper.enqueue <- pack:
					case <-time.After(100 * time.Millisecond):
						// drop packet; note there may be gaps in SeqNum on Asap b/c of this.
					case <-r.Halt.ReqStop.Chan:
						///p("r.Halt.ReqStop.Chan closed, returning.")
						return
					}
				}

				if pack.SeqNum > r.LargestSeqnoRcvd {
					r.LargestSeqnoRcvd = pack.SeqNum
					if pack.CumulBytesTransmitted < r.MaxCumulBytesTrans {
						panic("invariant that pack.CumulBytesTransmitted >= r.MaxCumulBytesTrans failed.")
					}
					r.MaxCumulBytesTrans = pack.CumulBytesTransmitted
				}
				if pack.CumulBytesTransmitted > r.MaxCumulBytesTrans {
					panic("invariant that pack.CumulBytesTransmitted goes in packet SeqNum order failed.")
				}

				// stuff has changed, so update
				r.UpdateControl(pack)
				// and tell snd about the new flow-control info
				//p("%v tellng r.snd.GotAck <- pack: '%#v'", r.Inbox, pack)
				cp := CopyPacketSansData(pack)
				select {
				case r.snd.GotPack <- cp:
				case <-r.Halt.ReqStop.Chan:
					///p("r.Halt.ReqStop.Chan closed, returning.")
					return
				}

				// data, or info?
				if pack.TcpEvent != EventData {
					// info:
					act := r.TcpState.UpdateTcp(pack.TcpEvent)
					err := r.doTcpAction(act, pack)
					if err == nil {
						continue recvloop
					} else {
						// err != nil is our indicator to exit
						///p("doTcpAction returned err='%v', returning.", err)
						return
					}
				}
				// data: actual data received, receiver side stuff follows.

				// if not old dup, add to hash of to-be-consumed
				if pack.SeqNum >= r.NextFrameExpected {
					r.RcvdButNotConsumed[pack.SeqNum] = pack
					//p("%v adding to r.RcvdButNotConsumed pack.SeqNum=%v   ... summary: %s",
					//r.Inbox, pack.SeqNum, r.HeldAsString())
				}

				//p("len r.Rxq = %v", len(r.Rxq))
				//p("pack=%#v", pack)
				//p("pack.SeqNum=%v, r.RecvWindowSize=%v, pack.SeqNum%%r.RecvWindowSize=%v", pack.SeqNum, r.RecvWindowSize, pack.SeqNum%r.RecvWindowSize)
				slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
				if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
					// Variation from textbook TCP: In the
					// presence of packet loss, if we drop certain packets,
					// the sender may re-try forever if we have non-overlapping windows.
					// So we'll ack out of bounds known good values anyway.
					// We could also do every K-th discard, but we want to get
					// the flow control ramp-up-from-zero correct and not acking
					// may inhibit that.
					//
					// Notice that UDT went to time-based acks; acking only every k milliseconds.
					// We may wish to experiment with that.
					//
					//p("%v pack.SeqNum %v outside receiver's window [%v, %v], dropping it",
					//	r.Inbox, pack.SeqNum, r.NextFrameExpected,
					//	r.NextFrameExpected+r.RecvWindowSize-1)
					r.DiscardCount++
					r.ack(r.LastFrameClientConsumed, pack, EventDataAck)
					continue recvloop
				}
				slot.Received = true
				slot.Pack = pack
				//p("%v packet %#v queued for ordered delivery, checking to see if we can deliver now",
				//	r.Inbox, slot.Pack)

				if pack.SeqNum == r.NextFrameExpected {
					// horray, we can deliver one or more frames in order

					//p("%v packet.SeqNum %v matches r.NextFrameExpected",
					//	r.Inbox, pack.SeqNum)
					for slot.Received {

						//p("%v actual in-order receive happening for SeqNum %v",
						//	r.Inbox, slot.Pack.SeqNum)

						r.ReadyForDelivery = append(r.ReadyForDelivery, slot.Pack)
						r.RecvHistory = append(r.RecvHistory, slot.Pack)
						//p("%v r.RecvHistory now has length %v", r.Inbox, len(r.RecvHistory))

						slot.Received = false
						slot.Pack = nil
						r.NextFrameExpected++
						slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
					}

					// update senders view of NextFrameExpected, for keep-alives.
					r.snd.SetRecvLastFrameClientConsumed(r.LastFrameClientConsumed)

					// not here, wait until delivered to consumer:
					// r.ack(r.LastFrameClientConsumed, pack, EventDataAck)
				} else {
					//p("%v packet SeqNum %v was not NextFrameExpected %v; stored packet but not delivered.",
					//	r.Inbox, pack.SeqNum, r.NextFrameExpected)
				}
			}
		}
	}()
	return nil
}

// UpdateFlowControl updates our flow control
// parameters r.LastAvailReaderMsgCap and
// r.LastAvailReaderBytesCap based on the
// most recently observed Packet deliveried
// status, and tells the sender about this
// indirectly with an r.snd.FlowCt.UpdateFlow()
// update.
//
// Called with each pack that RecvState
// receives, which is passed to UpdateFlow().
//
func (r *RecvState) UpdateControl(pack *Packet) {
	//begVal := r.LastAvailReaderMsgCap // used in diagnostic print at bottom

	// just like TCP flow control, where
	// advertisedWindow = maxRecvBuffer - (lastByteRcvd - nextByteRead)
	r.LastAvailReaderMsgCap = r.RecvWindowSize - (r.LargestSeqnoRcvd - r.LastMsgConsumed)
	r.LastAvailReaderBytesCap = r.RecvWindowSizeBytes - (r.MaxCumulBytesTrans - (r.LastByteConsumed + 1))
	r.snd.FlowCt.UpdateFlow(r.Inbox+":recver", r.Net, r.LastAvailReaderMsgCap, r.LastAvailReaderBytesCap, pack)

	//p("%v UpdateFlowControl in RecvState, bottom: "+
	//	"r.LastAvailReaderMsgCap= %v -> %v",
	//	r.Inbox, begVal, r.LastAvailReaderMsgCap)
}

// ack is a helper function, used in the recvloop above.
// Currently seqno is typically r.LastFrameClientConsumed
func (r *RecvState) ack(seqno int64, pack *Packet, event TcpEvent) {
	if pack.SeqNum < 0 {
		// keepalives will have seqno negative, so don't freak out.
	}
	///p("%s RecvState.ack() is doing ack with event '%s' in state '%s', giving seqno=%v. LastFrameClientConsumed=%v", r.Inbox, event, r.TcpState, seqno, r.LastFrameClientConsumed)

	r.UpdateControl(pack)
	///p("%v about to ack with AckNum: %v to %v, sending in the ack TcpEvent: %s", r.Inbox, seqno, pack.From, event)

	// send ack
	now := r.Clk.Now()
	ack := &Packet{
		From:                r.Inbox,
		Dest:                pack.From,
		SeqNum:              -99, // => ack flag
		SeqRetry:            -99,
		AckNum:              seqno,
		TcpEvent:            event,
		AvailReaderBytesCap: r.LastAvailReaderBytesCap,
		AvailReaderMsgCap:   r.LastAvailReaderMsgCap,
		AckRetry:            pack.SeqRetry,
		AckReplyTm:          now,
		DataSendTm:          pack.DataSendTm,
	}
	if len(r.snd.SendAck) == cap(r.snd.SendAck) {
		p("warning: %s ack queue is at capacity, very bad!  dropping oldest ack packet so as to add this one AckNum:%v, with TcpEvent:%s.", r.Inbox, ack.AckNum, ack.TcpEvent)
		<-r.snd.SendAck
	}
	select {
	case r.snd.SendAck <- ack: // hung here
	case <-time.After(time.Second * 10):
		panic(fmt.Sprintf("%s receiver could not inform sender of ack after 10 seconds, something is seriously wrong internally--deadlock most likely. dropping ack packet AckNum:%v, with TcpEvent:%s.", r.Inbox, ack.AckNum, ack.TcpEvent))
	case <-r.Halt.ReqStop.Chan:
	}
}

// Stop the RecvState componennt
func (r *RecvState) Stop() {
	r.Halt.ReqStop.Close()
	<-r.Halt.Done.Chan
}

// HeldAsString turns r.RcvdButNotConsumed into
// a string for convenience of display.
func (r *RecvState) HeldAsString() string {
	s := ""
	for sn := range r.RcvdButNotConsumed {
		s += fmt.Sprintf("%v, ", sn)
	}
	return s
}

func (r *RecvState) cleanupOnExit() {
	//p("RecvState.Inbox:%v cleanupOnExit is executing...", r.Inbox)
	if r.asapHelper != nil {
		r.asapHelper.Stop()
	}
}

// testModeOn idempotently turns
// on the testing mode. The testing
// mode can be checked at any point by testing if
// r.testing is non-nil.
func (r *RecvState) testModeOn() {
	if r.testing == nil {
		r.testing = &testCfg{}
	}
}

// testCfg is used for testing/advancing
// the SimClock automatically
type testCfg struct {
	ackCb                   ackCallbackFunc
	incrementClockOnReceive bool
}

func CopyPacketSansData(p *Packet) *Packet {
	cp := *p
	cp.Data = nil
	return &cp
}

func Blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	panicOn(err)
	h.Write(by)
	return []byte(h.Sum(nil))
}

func (r *RecvState) doTcpAction(act TcpAction, pack *Packet) error {
	///p("%s doTcpAction received action '%s' in state '%s', in response to event '%s'", r.Inbox, act, r.TcpState, pack.TcpEvent)
	switch act {
	case NoAction:
		return nil
	case SendDataAck:
		r.ack(r.LastFrameClientConsumed, pack, EventDataAck)
	case SendSyn:
		r.ack(r.LastFrameClientConsumed, pack, EventSyn)
	case SendSynAck:
		r.ack(r.LastFrameClientConsumed, pack, EventSynAck)
	case SendEstabAck:
		r.ack(r.LastFrameClientConsumed, pack, EventEstabAck)
	case SendFin:
		r.ack(r.LastFrameClientConsumed, pack, EventFin)
	case SendFinAck:
		r.ack(r.LastFrameClientConsumed, pack, EventFinAck)
	case DoAppClose:
		// after App has closed, then SendFinAck
		if r.AppCloseCallback != nil {
			r.AppCloseCallback()
		}
		r.ack(r.LastFrameClientConsumed, pack, EventFinAck)
	default:
		panic(fmt.Sprintf("unrecognized TcpAction act = %v", act))
	}
	return nil
}

func (r *RecvState) fillAsMuchAsPossible(rr *ReadRequest) {

	lenp := len(rr.P)
	if lenp == 0 {
		return
	}

	// copy our slice to ready, so our changing r.ReadyForDelivery
	// won't mess up the for range loop.
	ready := r.ReadyForDelivery

	// lastPack is the last completely consumed packet.
	var lastPack *Packet

	for _, pk := range ready {
		lendata := len(pk.Data) - pk.DataOffset
		m := copy(rr.P[rr.N:], pk.Data[pk.DataOffset:])
		rr.N += m
		pk.DataOffset += m

		// how far did we get?
		if m == lendata {
			// consumed the complete pk Packet
			r.ReadyForDelivery = r.ReadyForDelivery[1:]
			delete(r.RcvdButNotConsumed, pk.SeqNum)
			r.LastMsgConsumed = pk.SeqNum
			r.LastFrameClientConsumed = pk.SeqNum
			lastPack = pk

			// not the same as <- delivery, so check this:
			r.LastByteConsumed = pk.CumulBytesTransmitted
		} else {
			// partial packet consumed
			r.LastByteConsumed = pk.CumulBytesTransmitted - int64(lendata) + int64(m)
		}
		// is there space left in rr.P ?
		if rr.N >= lenp {
			// nope
			break
		}
		// yep, there is space in rr.P, continue
	}
	if lastPack != nil {
		r.ack(r.LastFrameClientConsumed, lastPack, EventDataAck)
	}
}
