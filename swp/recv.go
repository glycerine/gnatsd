package swp

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b
	"github.com/glycerine/idem"
)

var ErrShutdown = fmt.Errorf("shutdown in progress")
var ErrConnectWhenNotListen = fmt.Errorf("connect request when receiver was not in Listen state")

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
	RemoteInbox             string
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

	DoSendClosingCh chan *closeReq
	RecvSz          int64
	DiscardCount    int64

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
	retry    tcpRetryLogic

	AppCloseCallback  func()
	AcceptReadRequest chan *ReadRequest
	ConnectCh         chan *ConnectReq

	LocalSessNonce  string
	RemoteSessNonce string

	connReqPending  *ConnectReq
	retryTimerCh    <-chan time.Time
	tcpStateQueryCh chan TcpState

	KeepAliveInterval time.Duration
	keepAlive         <-chan time.Time
}

// InOrderSeq represents ordered (and gapless)
// data as delivered to the consumer application.
// The appliation requests
// it by asking on the Session.ReadMessagesCh channel.
type InOrderSeq struct {
	Seq []*Packet
}

// NewRecvState makes a new RecvState manager.
func NewRecvState(
	net Network,
	recvSz int64,
	recvSzBytes int64,
	timeout time.Duration,
	inbox string,
	snd *SenderState,
	clk Clock,
	nonce string,
	destInbox string,
	keepAliveInterval time.Duration,

) *RecvState {

	r := &RecvState{
		LocalSessNonce:      nonce,
		Clk:                 clk,
		Net:                 net,
		Inbox:               inbox,
		RemoteInbox:         destInbox,
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
		DoSendClosingCh:     make(chan *closeReq),
		LastMsgConsumed:     -1,
		LargestSeqnoRcvd:    -1,
		MaxCumulBytesTrans:  0,
		LastByteConsumed:    -1,
		NumHeldMessages:     make(chan int64),
		setAsapHelper:       make(chan *AsapHelper),
		TcpState:            Listen,
		AcceptReadRequest:   make(chan *ReadRequest),
		ConnectCh:           make(chan *ConnectReq),
		tcpStateQueryCh:     make(chan TcpState),

		// send keepalives (important especially for resuming flow from a
		// stopped state) at least this often:
		KeepAliveInterval: keepAliveInterval,
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
		// NB: we have to reserve somewhat *more* than than data
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
			mylog.Printf("%s RecvState defer/shutdown happening.", r.Inbox)
			stacktrace := make([]byte, 1<<20)
			length := runtime.Stack(stacktrace, true)
			mylog.Printf("full stack during RecvState defer:\n %s\n",
				string(stacktrace[:length]))

			r.Halt.RequestStop()
			r.Halt.MarkDone()
			r.cleanupOnExit()
			if r.snd != nil && r.snd.Halt != nil {
				r.snd.Halt.RequestStop()
			}
			r.Net.Close() // cleanup subscription of network
		}()
		mylog.Printf("%s RecvState.Start() running.", r.Inbox)

		// send keepalives (for resuming flow from a
		// stopped state) at least this often:
		r.keepAlive = time.After(r.KeepAliveInterval)

	recvloop:
		for {
			//p("%v top of recvloop, receiver NFE: %v. TcpState=%s",
			//	r.Inbox, r.NextFrameExpected, r.TcpState)

			deliverToConsumer = nil
			if len(r.ReadyForDelivery) > 0 {
				delivery.Seq = r.ReadyForDelivery
				deliverToConsumer = r.ReadMessagesCh

				//deliveryLen := len(delivery.Seq)
				//p("%v recloop has len %v r.ReadyForDelivery, SeqNum from [%v, %v]", r.Inbox, len(r.ReadyForDelivery), delivery.Seq[0].SeqNum, delivery.Seq[deliveryLen-1].SeqNum)
			}

			//p("%s recvloop: about to select", r.Inbox)
			select {
			case r.tcpStateQueryCh <- r.TcpState:
				// nothing more

			case <-r.keepAlive:
				select {
				case r.snd.keepAliveWithState <- r.TcpState:
					// sender now has r.TcpState, and will
					// communicate it in the keepalive packet.
				case <-r.Halt.ReqStop.Chan:
					return
				}
				r.keepAlive = time.After(r.KeepAliveInterval)

			case zr := <-r.DoSendClosingCh:
				//p("%s 1st recv got r.DoSendClosingCh <- true", r.Inbox)
				select {
				case r.snd.DoSendClosingCh <- zr:
					//p("%s 2nd recv sent to snd DoSendClosingCh <- true", r.Inbox)
				case <-r.Halt.ReqStop.Chan:
					return
				}
			case <-r.retryTimerCh:
				r.retryCheck()

			case cr := <-r.ConnectCh:
				//
				// Connect. Do an active open. Send syn to remote and
				// transition to SynSent state.
				//
				// Also here: if original SYN was lost, we'll end up
				// here again trying to re-send SYN, so don't freak.
				//
				if r.RemoteInbox == "" {

					mylog.Printf("recv.go: first time lock. setting r.RemoteInbox from connectReq.DestInbox = '%s'", cr.DestInbox)

					r.RemoteInbox = cr.DestInbox

				} else {

					if r.RemoteInbox != cr.DestInbox {
						cr.Err = fmt.Errorf("error: receiver already locked onto remote '%s', says receiver '%s', can't do '%s' from ConnectCh request.", r.RemoteInbox, r.Inbox, cr.DestInbox)
						close(cr.Done)
						continue
					}
				}

				if r.TcpState != Listen &&
					r.TcpState != SynSent {

					if r.TcpState == Established || r.TcpState == SynReceived {
						// no error, we are fine; connection is already established.
					} else {
						cr.Err = ErrConnectWhenNotListen
					}
					close(cr.Done)
					// ignore
					//p("continuing after rejecting connect request b/c not in listen or closed or SynSent state=%s", r.TcpState)
					continue
				}

				//p("%s we are setting r.connReqPending, and sending SYN.", r.Inbox)
				r.connReqPending = cr

				// send syn
				syn := &Packet{
					From:          r.Inbox,
					FromSessNonce: r.LocalSessNonce,
					Dest:          cr.DestInbox,
					SeqNum:        -98, // => syn flag
					SeqRetry:      -98,
					TcpEvent:      EventSyn,
				}
				cr.synPack = syn

				select {
				case r.snd.sendSynCh <- cr:
					//p("got connect request cr over the r.snd.sendSynCh")
				case <-r.Halt.ReqStop.Chan:
					//p("%v recvloop sees ReqStop waiting to sendSynCh, shutting down. [1]", r.Inbox)
					return
				}
				// The key state change.
				r.TcpState = SynSent
				//p("%s recv is now in SynSent, after telling sender to send syn to cr.DestInbox='%s'", r.Inbox, cr.DestInbox)

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
				//p("%v recvloop sees ReqStop waiting on r.MsgRecv, shutting down. [2]", r.Inbox)
				return
			case <-r.snd.SenderShutdown:
				//p("recvloop: got <-r.snd.SenderShutdown. note that len(r.RcvdButNotConsumed)=%v", len(r.RcvdButNotConsumed))
				return
			case pack := <-r.MsgRecv:
				//p("%v recvloop (in state '%s') sees packet.SeqNum '%v', event:'%s', AckNum:%v", r.Inbox, r.TcpState, pack.SeqNum, pack.TcpEvent, pack.AckNum)

				if pack.TcpEvent == EventSyn &&
					(r.TcpState == Fresh ||
						r.TcpState == Listen) {
					// track the first remote to
					// send us SYN and and lock onto it.
					if r.RemoteInbox == "" && pack.From != "" {
						mylog.Printf("%s recv.go locking onto remote %s. we are in state '%s'", r.Inbox, pack.From, r.TcpState)

						r.RemoteInbox = pack.From
						r.RemoteSessNonce = pack.FromSessNonce
					}
				}
				if r.RemoteInbox != "" && pack.From != r.RemoteInbox {
					// drop other remotes,
					// also enforcing that we see Syn 1st.
					mylog.Printf("%s dropping pack that isn't from '%s'", r.Inbox, r.RemoteInbox)
					continue
				}

				// drop non-session packets: they are for other sessions
				if (pack.DestSessNonce != "" || r.TcpState >= Established) &&
					pack.DestSessNonce != r.LocalSessNonce {
					//mylog.Printf("warning %v pack.DestSessNonce('%s') != r.LocalSessNonce('%s'): recvloop (in TcpState==%s) dropping packet.SeqNum '%v', event:'%s', AckNum:%v", r.Inbox, pack.DestSessNonce, r.LocalSessNonce, r.TcpState, pack.SeqNum, pack.TcpEvent, pack.AckNum)
					continue // drop others
				}
				if r.RemoteSessNonce != "" &&
					pack.FromSessNonce != r.RemoteSessNonce {
					//mylog.Printf("warining %v pack.FromSessNonce('%s') != r.RemoteSessNonce('%s'): recvloop (in TcpState==%s) dropping packet.SeqNum '%v', event:'%s', AckNum:%v", r.Inbox, pack.FromSessNonce, r.RemoteSessNonce, pack.SeqNum, r.TcpState, pack.TcpEvent, pack.AckNum)
					continue // drop others
				}

				// test instrumentation, used e.g. in clock_test.go
				if r.testing != nil && r.testing.incrementClockOnReceive {
					r.Clk.(*SimClock).Advance(time.Second)
				}

				if len(pack.Data) > 0 {
					chk := Blake2bOfBytes(pack.Data)
					if 0 != bytes.Compare(pack.Blake2bChecksum, chk) {
						mylog.Printf("expected checksum to be '%x', but was '%x'. For pack.SeqNum %v",
							pack.Blake2bChecksum, chk, pack.SeqNum)
						//panic("data corruption detected by blake2b checksum")
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
					event := pack.TcpEvent
					fromState := pack.FromTcpState
				doNextEvent:
					preUpdate := r.TcpState
					//p("%s recvp about to UpdateTcp, starting state=%s", r.Inbox, r.TcpState)
					act := r.TcpState.UpdateTcp(event, fromState)
					//p("%s recvp done with UpdateTcp, state is now=%s", r.Inbox, r.TcpState)
					if preUpdate != r.TcpState {
						r.setupRetry(preUpdate, r.TcpState, pack, act)
					}

					eventNext, err := r.doTcpAction(act, pack)
					//p("%s recvp done with doTcpAction(act=%s), eventNext=%s. err=%v", r.Inbox, act, eventNext, err)

					// eventNext is typically EventNil, except when
					// we need to sendFin after application close and
					// enter LastAck. We still want to capture that
					// in the retry logic, so we use a doNextEvent.
					if eventNext != EventNil {
						event = eventNext
						goto doNextEvent
					}
					if err != nil {
						// err != nil is our indicator to exit
						//p("doTcpAction returned err='%v', returning.", err)
						return
					}
					if r.TcpState == Closed {
						//p("%s receiver recognized r.TcpState == Closed, shutting down", r.Inbox)
						return
					}
					continue recvloop
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
// Currently seqno is typically r.LastFrameClientConsumed.
//
// Allow pack to be nil for final death gasp EventReset.
func (r *RecvState) ack(seqno int64, pack *Packet, event TcpEvent) {

	// keepalives will have seqno negative, so don't freak out.

	///p("%s RecvState.ack() is doing ack with event '%s' in state '%s', giving seqno=%v. LastFrameClientConsumed=%v", r.Inbox, event, r.TcpState, seqno, r.LastFrameClientConsumed)

	if pack != nil {
		r.UpdateControl(pack)
	}
	///p("%v about to ack with AckNum: %v to %v, sending in the ack TcpEvent: %s", r.Inbox, seqno, pack.From, event)

	// send ack
	now := r.Clk.Now()
	var ackRetry int64 = -1
	dataSendTm := now
	if pack != nil {
		ackRetry = pack.SeqRetry
		dataSendTm = pack.DataSendTm
	}
	ack := &Packet{
		From:                r.Inbox,
		FromSessNonce:       r.LocalSessNonce,
		Dest:                r.RemoteInbox,
		DestSessNonce:       r.RemoteSessNonce,
		SeqNum:              -99, // => ack flag
		SeqRetry:            -99,
		AckNum:              seqno,
		TcpEvent:            event,
		AvailReaderBytesCap: r.LastAvailReaderBytesCap,
		AvailReaderMsgCap:   r.LastAvailReaderMsgCap,
		AckRetry:            ackRetry,
		AckReplyTm:          now,
		DataSendTm:          dataSendTm,
	}
	if len(r.snd.SendAck) == cap(r.snd.SendAck) {
		mylog.Printf("warning: %s ack queue is at capacity, very bad!  dropping oldest ack packet so as to add this one AckNum:%v, with TcpEvent:%s.", r.Inbox, ack.AckNum, ack.TcpEvent)

		// discard first to make room:
		<-r.snd.SendAck
	}
	select {
	case r.snd.SendAck <- ack:
	case <-time.After(time.Second * 10):
		mylog.Printf("%s receiver could not inform sender of ack after 10 seconds, something is seriously wrong internally--deadlock most likely. dropping ack packet AckNum:%v, with TcpEvent:%s.", r.Inbox, ack.AckNum, ack.TcpEvent)
	case <-r.Halt.ReqStop.Chan:
	}
}

// Stop the RecvState componennt
func (r *RecvState) Stop() {
	//p("%v RecvState.Stop() called.", r.Inbox)
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

func (r *RecvState) doTcpAction(act TcpAction, pack *Packet) (TcpEvent, error) {
	//p("%s doTcpAction received action '%s' in state '%s', in response to event '%s'", r.Inbox, act, r.TcpState, pack.TcpEvent)

	switch act {

	case NoAction:
		return EventNil, nil

	case SendDataAck:
		r.ack(r.LastFrameClientConsumed, pack, EventDataAck)

	case SendSyn:
		r.ack(r.LastFrameClientConsumed, pack, EventSyn)

	case SendSynAck:
		// server learns of the remote session nonce.

		// allow repeats...
		if r.RemoteSessNonce != "" &&
			r.RemoteSessNonce != pack.FromSessNonce {
			panic(fmt.Sprintf("r.RemoteSessNonce is "+
				"already set to '%s', this pack has '%s'",
				r.RemoteSessNonce, pack.FromSessNonce))
		}
		r.RemoteSessNonce = pack.FromSessNonce
		r.ack(r.LastFrameClientConsumed, pack, EventSynAck)

	case SendEstabAck:
		// client learns of the remote session nonce
		if r.RemoteSessNonce != "" {
			panic(fmt.Sprintf("r.RemoteSessNonce "+
				"is already set '%s'", r.RemoteSessNonce))
		}
		r.RemoteSessNonce = pack.FromSessNonce
		if r.connReqPending == nil {
			panic("must have pending connReqPending " +
				"when doing SendEstabAck, but did not.")
		}
		r.connReqPending.RemoteNonce = r.RemoteSessNonce
		close(r.connReqPending.Done)
		r.connReqPending = nil
		r.ack(r.LastFrameClientConsumed, pack, EventEstabAck)

	case SendFin:
		r.ack(r.LastFrameClientConsumed, pack, EventFin)

	case SendFinAck:
		r.ack(r.LastFrameClientConsumed, pack, EventFinAck)

	case DoAppClose:
		// don't ack until app is shutdown.
		if r.AppCloseCallback != nil {
			// Only do this once.
			r.AppCloseCallback()

			// If we callback more than once, the app may
			// try to double close channels etc.
			// So prevent a repeat callback.
			r.AppCloseCallback = nil
		}
		r.ack(r.LastFrameClientConsumed, pack, EventFinAck)
		return EventApplicationClosed, nil

	default:
		panic(fmt.Sprintf("unrecognized TcpAction act = %v", act))
	}
	return EventNil, nil
}

func (r *RecvState) fillAsMuchAsPossible(rr *ReadRequest) {
	lenp := len(rr.P)
	//p("RecvState.fillAsMuchAsPossible() top. with len(rr.P)=%v", lenp)
	if lenp == 0 {
		return
	}

	// copy our slice to ready, so our changing r.ReadyForDelivery
	// won't mess up the for range loop.
	ready := r.ReadyForDelivery
	//p("RecvState.fillAsMuchAsPossible(): len(ready)=%v", len(ready))

	// lastPack is the last completely consumed packet.
	var lastPack *Packet

	for _, pk := range ready {
		lendata := len(pk.Data) - pk.DataOffset
		//p("fillAsMuch: next packet pk is of len %v", lendata)
		m := copy(rr.P[rr.N:], pk.Data[pk.DataOffset:])
		rr.N += m
		pk.DataOffset += m

		// how far did we get?
		if m == lendata {
			//p("consumed complete packet k=%v", k)
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
			//p("partial packet consumed on k=%v", k)
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

type ConnectReq struct {
	DestInbox   string
	Err         error
	RemoteNonce string
	Done        chan bool

	synPack *Packet
}

func NewConnectReq(dest string) *ConnectReq {
	return &ConnectReq{
		DestInbox: dest,
		Done:      make(chan bool),
	}
}

func (r *RecvState) Connect(dest string, simulateUnderTestLostSynCount int, timeout time.Duration, numAttempts int) (remoteNonce string, err error) {

	over := timeout * time.Duration(numAttempts)
	overallTooLong := time.After(over)
	tries := 0
	for {
		tries++
		cr := NewConnectReq(dest)

		select {
		case r.ConnectCh <- cr:
		case <-time.After(timeout):
			return "", fmt.Errorf("Connect() timeout waiting for Syn, after %v", timeout)
		case <-r.Halt.ReqStop.Chan:
			return "", ErrShutdown
		}

		// simulate lost SYN and have to retry:
		if tries <= simulateUnderTestLostSynCount {
			continue
		}
		t0 := time.Now()
		select {
		case <-time.After(timeout):
			mylog.Printf("connect request: no answer after %v, trying again to connect to dest '%s'", timeout, dest)
			// try again
			continue
		case <-overallTooLong:
			return "", fmt.Errorf("r.Connect() timeout waiting to SynAck, after over=%v wait.", over)
		case <-cr.Done:
			mylog.Printf("r.Connect(dest='%s') completed in %v. with cr.Err='%v' and cr.RemoteNonce='%s'", dest, time.Since(t0), cr.Err, cr.RemoteNonce)

			return cr.RemoteNonce, cr.Err
		case <-r.Halt.ReqStop.Chan:
			return "", ErrShutdown
		}

		return "", ErrShutdown
	}
}

var ErrTimeoutClose = fmt.Errorf("timed-out after 10 seconds waiting to get Close() message through.")

// Close is gentler than Stop(). It politely notifies the remote side
// to go through its full shutdown sequence. It will
// return before that sequence is complete.
func (r *RecvState) Close(zr *closeReq) error {
	//p("%s RecvState.Close() running", r.Inbox)

	select {
	case r.DoSendClosingCh <- zr:
		select {
		case <-zr.done:
			return nil
		case <-r.Halt.ReqStop.Chan:
			return nil
		case <-time.After(10 * time.Second):
			return ErrTimeoutClose
		}
	case <-r.Halt.ReqStop.Chan:
		return nil
	case <-time.After(10 * time.Second):
		return ErrTimeoutClose
	}
	return nil
}
