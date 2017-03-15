package swp

import (
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glycerine/idem"
)

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	OrigSendTime  time.Time
	RetryDeadline time.Time
	RetryDur      time.Duration
	Pack          *Packet
}

func (s *TxqSlot) String() string {
	return fmt.Sprintf("TxqSlot{RetryDeadline: %v, Pack.SeqNum:%v, RetryDur:%v}", s.RetryDeadline, s.Pack.SeqNum, s.RetryDur)
}

// SenderState tracks the sender's sliding window state.
// To avoid circular deadlocks, the SenderState never talks
// directly to the RecvState. The RecvState will
// tell the Sender stuff on GotPack.
//
// Acks, retries, keep-alives, and original-data: these
// are the four types of sends we do.
// Also now a closing message is sent on shutdown.
//
type SenderState struct {
	Clk              Clock
	Net              Network
	Inbox            string
	Dest             string
	LastAckRec       int64
	LastFrameSent    int64
	Txq              []*TxqSlot
	SenderWindowSize int64
	mut              sync.Mutex
	Timeout          time.Duration

	// the main goroutine safe way to request
	// sending a packet:
	BlockingSend chan *Packet

	GotPack chan *Packet

	Halt         *idem.Halter
	SendHistory  []*Packet
	SendSz       int64
	SendAck      chan *Packet
	DiscardCount int64

	LastSendTime            time.Time
	LastHeardFromDownstream time.Time
	KeepAliveInterval       time.Duration
	keepAlive               <-chan time.Time

	// after this many failed keepalives, we
	// close down the session. Set to less than 1
	// to disable the auto-close.
	NumFailedKeepAlivesBeforeClosing int

	SentButNotAckedByDeadline *retree
	SentButNotAckedBySeqNum   *retree

	// flow control params
	// last seen from our downstream
	// receiver, we throttle ourselves
	// based on these.
	LastSeenAvailReaderBytesCap int64
	LastSeenAvailReaderMsgCap   int64

	// do synchronized access via GetFlow()
	// and UpdateFlow(s.Net)
	FlowCt                 *FlowCtrl
	TotalBytesSent         int64
	TotalBytesSentAndAcked int64
	rtt                    *RTT

	// nil after Stop() unless we terminated the session
	// due to too many outstanding acks
	exitErr error

	// tell the receiver that sender is terminating
	SenderShutdown chan bool

	recvLastFrameClientConsumed int64
}

func (s *SenderState) GetRecvLastFrameClientConsumed() int64 {
	return atomic.LoadInt64(&s.recvLastFrameClientConsumed)
}
func (s *SenderState) SetRecvLastFrameClientConsumed(nfe int64) {
	atomic.StoreInt64(&s.recvLastFrameClientConsumed, nfe)
}

// NewSenderState constructs a new SenderState struct.
func NewSenderState(net Network, sendSz int64, timeout time.Duration,
	inbox string, destInbox string, clk Clock, keepAliveInterval time.Duration) *SenderState {
	s := &SenderState{
		Clk:                       clk,
		Net:                       net,
		Inbox:                     inbox,
		Dest:                      destInbox,
		SenderWindowSize:          sendSz,
		Txq:                       make([]*TxqSlot, sendSz),
		Timeout:                   timeout,
		LastFrameSent:             -1,
		LastAckRec:                -1,
		Halt:                      idem.NewHalter(),
		SendHistory:               make([]*Packet, 0),
		BlockingSend:              make(chan *Packet),
		SendSz:                    sendSz,
		GotPack:                   make(chan *Packet),
		SendAck:                   make(chan *Packet, 5), // buffered so we don't deadlock
		SentButNotAckedByDeadline: newRetree(compareRetryDeadline),
		SentButNotAckedBySeqNum:   newRetree(compareSeqNum),

		SenderShutdown: make(chan bool),

		// send keepalives (important especially for resuming flow from a
		// stopped state) at least this often:
		KeepAliveInterval: keepAliveInterval,
		FlowCt: &FlowCtrl{Flow: Flow{
			// Control messages such as acks and keepalives
			// should not be blocked by flow-control (for
			// correctness/resumption from no-flow), so we need
			// to reserve extra headroom in the nats
			// subscription limits of this much to
			// allow resumption of flow.
			//
			// These reserved headroom settings can be
			// manually made larger before calling Start()
			//  -- and might need to be if you are running
			// very large windowMsgSz and/or windowByteSz; or
			// if you have large messages.
			ReservedByteCap: 64 * 1024,
			ReservedMsgCap:  32,
		}},
		// don't start fast, as we could overwhelm
		// the receiver. Instead start with sendSz,
		// assuming the receiver has a slot size just
		// like sender'sx. Notice that we'll need to allow
		// at least 2 messages in our 003 reorder test runs.
		// LastSeenAvailReaderMsgCap will get updated
		// after the first ack or keep alive to reflect
		// the actual receiver capacity. This is just
		// an inital value to use before we've heard from
		// the actual receiver over network.
		LastSeenAvailReaderMsgCap:   sendSz,
		LastSeenAvailReaderBytesCap: 1024 * 1024,
		rtt: NewRTT(),
		LastHeardFromDownstream: clk.Now(),
	}
	for i := range s.Txq {
		s.Txq[i] = &TxqSlot{}
	}
	return s
}

// ComputeInflight returns the number of bytes and messages
// that are in-flight: they have been sent but not yet acked.
func (s *SenderState) ComputeInflight() (bytesInflight int64, msgInflight int64) {
	for it := s.SentButNotAckedByDeadline.tree.Min(); !it.Limit(); it = it.Next() {
		slot := it.Item().(*TxqSlot)
		msgInflight++
		bytesInflight += int64(len(slot.Pack.Data))
	}
	return
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start(sess *Session) {

	go func() {

		var acceptSend chan *Packet

		// check for expired timers at wakeFreq
		wakeFreq := s.Timeout / 2

		// send keepalives (for resuming flow from a
		// stopped state) at least this often:
		s.keepAlive = time.After(s.KeepAliveInterval)

		regularIntervalWakeup := time.After(wakeFreq)

		// shutdown stuff, all in one place for consistency
		defer func() {
			///p("%s SendState defer/shutdown happening.", s.Inbox)
			s.doSendClosing()
			close(s.SenderShutdown) // stops the receiver
			s.Halt.ReqStop.Close()
			s.Halt.Done.Close()
			sess.Halt.Done.Close() // lets clients detect shutdown
		}()

	sendloop:
		for {
			//p("%v top of sendloop, sender LAR: %v, LFS: %v \n",
			//	s.Inbox, s.LastAckRec, s.LastFrameSent)

			// does the downstream reader have capacity to accept a send?
			// Block any new sends if so. We do a conditional receive. Start by
			// assuming no:
			acceptSend = nil

			// then check if we can set acceptSend.
			//
			// We accept a packet for sending if flow control info
			// from the receiver allows it.
			//
			bytesInflight, msgInflight := s.ComputeInflight()
			//p("%v bytesInflight = %v", s.Inbox, bytesInflight)
			//p("%v msgInflight = %v", s.Inbox, msgInflight)

			if s.LastSeenAvailReaderMsgCap-msgInflight > 0 &&
				s.LastSeenAvailReaderBytesCap-bytesInflight > 0 {
				//p("%v flow-control: okay to send. s.LastSeenAvailReaderMsgCap: %v > msgInflight: %v",
				//	s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight)
				acceptSend = s.BlockingSend
			} else {
				//p("%v flow-control kicked in: not sending. s.LastSeenAvailReaderMsgCap = %v,"+
				//	" msgInflight=%v, s.LastSeenAvailReaderBytesCap=%v bytesInflight=%v",
				//	s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight,
				//	s.LastSeenAvailReaderBytesCap, bytesInflight)
			}

			//p("%v top of sender select loop", s.Inbox)
			select {
			case <-s.keepAlive:
				//p("%v keepAlive at %v", s.Inbox, s.Clk.Now())
				s.doKeepAlive()

			case <-regularIntervalWakeup:
				now := s.Clk.Now()
				//p("%v regularIntervalWakeup at %v", s.Inbox, now)

				if s.NumFailedKeepAlivesBeforeClosing > 0 {
					thresh := s.KeepAliveInterval * time.Duration(s.NumFailedKeepAlivesBeforeClosing)
					//p("at regularInterval (every %v) doing check: SenderState.NumFailedKeepAlivesBeforeClosing=%v, checking for close after thresh %v (== %v * %v)", wakeFreq, s.NumFailedKeepAlivesBeforeClosing, thresh, s.KeepAliveInterval, s.NumFailedKeepAlivesBeforeClosing)
					elap := now.Sub(s.LastHeardFromDownstream)
					//p("elap = %v; s.LastHeardFromDownstream=%v", elap, s.LastHeardFromDownstream)
					if elap > thresh {

						// time to shutdown
						p("too long (%v) since we've heard from the other end, declaring session dead and closing it.", thresh)
						return
					}
				}

				// have any of our packets timed-out and need to be
				// sent again?
				retry := []*TxqSlot{}

				s.SentButNotAckedByDeadline.deleteThroughDeadline(now,
					func(slot *TxqSlot) {
						s.SentButNotAckedBySeqNum.deleteSlot(slot)

						retry = append(retry, slot)
						///p("%s sender detects slot with SeqNum=%v has retry deadline expired; since deadline = %v < %v == now, by %v. Elap since orig send time: %v. slot.RetryDur:%v", s.Inbox, slot.Pack.SeqNum, slot.RetryDeadline, now, now.Sub(slot.RetryDeadline), now.Sub(slot.OrigSendTime), slot.RetryDur)
					})
				if len(retry) > 0 {
					///p("%v sender retry list is len %v", s.Inbox, len(retry))
				}

				for _, slot := range retry {

					// reset deadline and resend
					now := s.Clk.Now()
					flow := s.FlowCt.UpdateFlow(s.Inbox, s.Net, -1, -1, nil)
					slot.RetryDur = s.GetDeadlineDur(flow)
					slot.RetryDeadline = now.Add(slot.RetryDur)
					slot.Pack.SeqRetry++
					slot.Pack.DataSendTm = now

					slot.Pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
					slot.Pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
					slot.Pack.FromRttEstNsec = int64(s.rtt.GetEstimate())
					slot.Pack.FromRttSdNsec = int64(s.rtt.GetSd())
					slot.Pack.FromRttN = s.rtt.N

					s.SentButNotAckedByDeadline.insert(slot)
					s.SentButNotAckedBySeqNum.insert(slot)

					///p("%v doing retry Net.Send() for pack.SeqNum = '%v' of paydirt len %v", s.Inbox, slot.Pack.SeqNum, len(slot.Pack.Data))
					err := s.Net.Send(slot.Pack, "retry")
					panicOn(err)
				}
				regularIntervalWakeup = time.After(wakeFreq)

			case <-s.Halt.ReqStop.Chan:
				return
			case pack := <-acceptSend:
				//p("%v got <-acceptSend pack: '%#v'", s.Inbox, pack)
				s.doOrigDataSend(pack)

			case a := <-s.GotPack:
				s.LastHeardFromDownstream = a.ArrivedAtDestTm

				// ack/keepalive/data packet received in 'a' -
				// do sender side stuff
				//
				//p("%v sender GotPack a: %#v", s.Inbox, a)
				//
				// flow control: respect a.AvailReaderBytesCap
				// and a.AvailReaderMsgCap info that we have
				// received from this ack
				//
				//p("%v sender GotPack, updating s.LastSeenAvailReaderMsgCap %v -> %v",
				//	s.Inbox, s.LastSeenAvailReaderMsgCap, a.AvailReaderMsgCap)
				s.LastSeenAvailReaderBytesCap = a.AvailReaderBytesCap
				s.LastSeenAvailReaderMsgCap = a.AvailReaderMsgCap

				s.UpdateRTT(a)

				// need to update our SentButNotAcked* trees
				// and remove everything before AckNum, which is cumulative.
				numDel := 0
				s.SentButNotAckedBySeqNum.deleteThroughSeqNum(
					a.AckNum, func(slot *TxqSlot) {
						s.SentButNotAckedByDeadline.deleteSlot(slot)
						numDel++
						if slot.Pack.CliAcked != nil {
							///p("got ack for packet that has CliAcked on it; a.AckNum=%v. len(Data)=%v. event=%s. clearing slot.Pack.SeqNum=%v", a.AckNum, len(slot.Pack.Data), a.TcpEvent, slot.Pack.SeqNum)
							if slot.Pack.CliAcked != nil {
								slot.Pack.CliAcked.Bcast(slot.Pack.SeqNum)
							}
						}
						///p("%s deleting slot.Pack.SeqNum=%v <= a.AckNum=%v from s.SentButNotAcked", s.Inbox, slot.Pack.SeqNum, a.AckNum)
						//s.TotalBytesSentAndAcked += int64(len(slot.Pack.Data))
						if slot.Pack.Accounting != nil {
							nba := atomic.LoadInt64(&slot.Pack.Accounting.NumBytesAcked)
							nba += int64(len(slot.Pack.Data))
							atomic.StoreInt64(&slot.Pack.Accounting.NumBytesAcked, nba)
						}
					})
				///p("%v after numDel %v through a.AckNum=%v, s.SentButNotAckedBySeqNum=\n%s\n, and s.SentButNotAckedByDeadline=\n%s\n", s.Inbox, numDel, a.AckNum, s.SentButNotAckedBySeqNum, s.SentButNotAckedByDeadline)

				// we were having problems with delete ByDeadline not
				// happening, so assert a sanity check here.
				lenBySeq := s.SentButNotAckedBySeqNum.tree.Len()
				lenByDeadline := s.SentButNotAckedByDeadline.tree.Len()
				if lenBySeq != lenByDeadline {
					panic(fmt.Sprintf("lenBySeq=%v, while lenByDeadline=%v", lenBySeq, lenByDeadline))
				}

				if a.TcpEvent != EventDataAck || a.AckNum < 0 {
					// it wasn't an Ack, just updated flow info
					// from a received data message; or a keepalive (a.AckNum < 0).
					//p("%s sender Gotack: just updated flow control, continuing sendloop", s.Inbox)
					continue sendloop
				}
				// INVAR: a.TcpEvent == EventDataAck

				///p("%s sender has EventDataAck(%v) or a.AckNum(%v) < 0 ...", s.Inbox, a.TcpEvent == EventDataAck, a.AckNum)
				if !InWindow(a.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					///p("%v a.AckNum = %v outside sender's window [%v, %v], dropping it.", s.Inbox, a.AckNum, s.LastAckRec+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				//p("%v packet.AckNum = %v inside sender's window, keeping it.", s.Inbox, a.AckNum)

			case ackPack := <-s.SendAck:
				// request to send an ack:
				// don't go though the BlockingSend protocol; since
				// could effectively livelock us.
				///p("%v doing ack Net.Send() where the ackPack has AckNum '%v'", s.Inbox, ackPack.AckNum)
				ackPack.FromRttEstNsec = int64(s.rtt.GetEstimate())
				ackPack.FromRttSdNsec = int64(s.rtt.GetSd())
				ackPack.FromRttN = s.rtt.N
				err := s.Net.Send(ackPack, "SendAck/ackPack")
				//panicOn(err) "nats: connection closed"
				if err != nil {
					return
				}
			}
		}
	}()
}

// Stop the SenderState componennt
func (s *SenderState) Stop() {
	s.Halt.RequestStop()
	<-s.Halt.Done.Chan
}

// doOrigDataSend() is for first time sends of data, not retries or acks.
// If getAck is true, then we will call Flush on the
// nats connection. This will wait for an ack from the
// server (or timeout after 60 seconds).
// Return the packet's sequence number, from
// the LastFrameSent counter.
func (s *SenderState) doOrigDataSend(pack *Packet) int64 {

	s.LastFrameSent++
	//p("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

	s.TotalBytesSent += int64(len(pack.Data))
	pack.CumulBytesTransmitted = s.TotalBytesSent

	lfs := s.LastFrameSent
	pos := lfs % s.SenderWindowSize
	slot := s.Txq[pos]

	if len(pack.Data) > 0 {
		pack.Blake2bChecksum = Blake2bOfBytes(pack.Data)
		//p("%v SenderState.send() added blake2b '%x' of len(pack.Data)=%v", s.Inbox, pack.Blake2bChecksum, len(pack.Data))
	}

	pack.SeqNum = lfs
	///p("%v sender in acceptSend, pack.SeqNum='%v'", s.Inbox, pack.SeqNum)

	if pack.From != s.Inbox {
		pack.From = s.Inbox
	}
	pack.From = s.Inbox
	slot.Pack = pack

	now := s.Clk.Now()
	s.SendHistory = append(s.SendHistory, pack)
	slot.OrigSendTime = now

	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	slot.RetryDur = s.GetDeadlineDur(flow)
	slot.RetryDeadline = now.Add(slot.RetryDur)
	s.LastSendTime = now

	// data sends get stored in the
	// SentButNotAcked trees.

	// These inserts MUST HAPPEN AFTER slot.RetryDeadline is
	// set above!
	// -- or else the sorting won't work right, and so
	// the delete won't work right.
	s.SentButNotAckedByDeadline.insert(slot)
	s.SentButNotAckedBySeqNum.insert(slot)

	///p("%v doSend(), after setting RetryDeadline on SeqNum=%v, slot = \n%s\n", s.Inbox, slot.Pack.SeqNum, slot)

	//p("%v doSend(), flow = '%#v'", s.Inbox, flow)
	pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
	pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
	pack.DataSendTm = now

	// tell Dest about our RTT estimate.
	pack.FromRttEstNsec = int64(s.rtt.GetEstimate())
	pack.FromRttSdNsec = int64(s.rtt.GetSd())
	pack.FromRttN = s.rtt.N

	err := s.Net.Send(slot.Pack, fmt.Sprintf("doOrigDataSend() for %v", s.Inbox))
	panicOn(err)

	return lfs
}

func (s *SenderState) doKeepAlive() {
	if time.Since(s.LastSendTime) < s.KeepAliveInterval {
		return
	}
	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	//p("%v doKeepAlive(), flow = '%#v'", s.Inbox, flow)
	// send a packet with no data, to elicit an ack
	// with a new advertised window. This is
	// *not* an ack, because we need it to be
	// acked itself so we get any updated
	// flow control info from the other end.
	now := s.Clk.Now()
	s.LastSendTime = now

	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		SeqNum:              -777, // => keepalive
		SeqRetry:            -777,
		DataSendTm:          now,
		AckNum:              s.GetRecvLastFrameClientConsumed(),
		AckRetry:            -777,
		TcpEvent:            EventKeepAlive,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,

		FromRttEstNsec: int64(s.rtt.GetEstimate()),
		FromRttSdNsec:  int64(s.rtt.GetSd()),
		FromRttN:       s.rtt.N,
	}
	//p("%v doing keepalive Net.Send()", s.Inbox)
	err := s.Net.Send(kap, fmt.Sprintf("keepalive from %v", s.Inbox))
	if err != nil {
		fmt.Fprintf(os.Stderr, "on send Keepalive attempt, got err = '%v'\n", err)
	}

	s.keepAlive = time.After(s.KeepAliveInterval)
}

// If close is out of bound, what if it is lost?
// Is only the inband data sequence retried? Or
// is out of band also retried? If close is not
// acked with the regular data seqnums, then we
// need to ack it separately.
//
// We seem to be closing before all data has
// been delivered to the far-end client. Should
// we be waiting to ack it until the client has
// actually read it?
//
// We are seeing some packets misordered... how
// is that possible if the sequence numbers are
// correct?
//
func (s *SenderState) doSendClosing() {
	//p("%s doSendClosing() running", s.Inbox)
	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	now := s.Clk.Now()
	s.LastSendTime = now
	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		SeqNum:              -888, // => endpoint is closing
		SeqRetry:            -888,
		DataSendTm:          now,
		AckRetry:            -888,
		TcpEvent:            EventFin,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,

		FromRttEstNsec: int64(s.rtt.GetEstimate()),
		FromRttSdNsec:  int64(s.rtt.GetSd()),
		FromRttN:       s.rtt.N,
	}
	//p("%v doing Closing Net.Send()", s.Inbox)
	err := s.Net.Send(kap, fmt.Sprintf("endpoint is closing, from %v", s.Inbox))
	//panicOn(err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "on send Closing attempt, got err = '%v'\n", err)
	}

	s.keepAlive = time.After(s.KeepAliveInterval)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (s *SenderState) UpdateRTT(pack *Packet) {
	// avoid clock skew between machines by
	// not sampling one-way elapsed times.
	if pack.TcpEvent == EventKeepAlive {
		return
	}
	if pack.TcpEvent != EventDataAck {
		return
	}
	//p("%v UpdateRTT top, pack = %#v", s.Inbox, pack)

	// acks have roundtrip times we can measure
	// use our own clock, thus avoiding clock
	// skew.

	obs := s.Clk.Now().Sub(pack.DataSendTm)

	// exclude obvious outliers where a round trip
	// took 60 seconds or more
	if obs > time.Minute {
		fmt.Printf("\n %v now: %v; UpdateRTT exluding outlier outside 60 seconds:"+
			" pack.DataSendTm = %v, observed rtt = %v.  pack = '%#v'\n",
			s.Inbox, s.Clk.Now(), pack.DataSendTm, obs, pack)
		return
	}

	//p("%v pack.DataSendTm = %v", s.Inbox, pack.DataSendTm)
	s.rtt.AddSample(obs)

	//sd := s.rtt.GetSd()
	//p("%v UpdateRTT: observed rtt was %v. new smoothed estimate after %v samples is %v. sd = %v", s.Inbox, obs, s.rtt.N, s.rtt.GetEstimate(), sd)
}

// GetDeadlineDur returns the duration until
// the receive deadline using a
// weighted average of our observed RTT info and the remote
// end's observed RTT info.
// Add it to time.Now() before using.
func (s *SenderState) GetDeadlineDur(flow Flow) time.Duration {
	var ema time.Duration
	var sd time.Duration
	var n int64 = s.rtt.N

	if s.rtt.N < 1 {
		if flow.RemoteRttN > 2 {
			ema = time.Duration(flow.RemoteRttEstNsec)
			sd = time.Duration(flow.RemoteRttSdNsec)
		} else {
			//p("nobody has good info, just guess. retry deadline will be 500msec out.")
			return 500 * time.Millisecond
		}
	} else {
		// we have at least one local round-trip sample

		// exponential moving average of observed RTT
		ema = s.rtt.GetEstimate()

		if s.rtt.N > 1 {
			sd = s.rtt.GetSd()
		} else {
			// default until we have 2 or more data points
			sd = ema
			// sanity check and cap if need be
			if sd > 2*ema {
				sd = 2 * ema
			}
		}
	}

	// blend local and remote info, in a weighted average.
	if flow.RemoteRttN > 2 && s.rtt.N > 2 {

		// Satterthwaite appoximation
		sd1 := float64(sd)
		var1 := sd1 * sd1
		n1 := float64(n)
		sd2 := float64(flow.RemoteRttSdNsec)
		var2 := sd2 * sd2
		n2 := float64(flow.RemoteRttN)
		SathSd := math.Sqrt((var1 / n1) + (var2 / n2))
		newSd := time.Duration(int64(SathSd))

		// variance weighted RTT estimate
		rtt1 := float64(ema)
		rtt2 := float64(flow.RemoteRttEstNsec)
		invvar1 := 1 / var1
		invvar2 := 1 / var2
		newEma := time.Duration(int64((rtt1*invvar1 + rtt2*invvar2) / (invvar1 + invvar2)))

		// update:
		//p("RTTema1=%v  n1=%v  sd1=%v    RTTemaRemote=%v  nRemote=%v  sdRemote=%v", rtt1, n1, sd1, rtt2, n2, sd2)
		//p("weighted average ema : %v -> %v", ema, newEma)
		//p("weighted average sd  : %v -> %v", sd, newSd)
		sd = newSd
		ema = newEma
	}

	// allow four standard deviations of margin
	// before consuming bandwidth for retry.
	fin := ema + 4*sd
	//p("%v ema is %v +/- %v", s.Inbox, ema, sd)

	if s.rtt.N < 10 {
		// minimum sanity thresh while small sample size
		minTo := time.Millisecond * 100
		if fin < minTo {
			fin = minTo
		}
	}
	//p("returning deadline of duration %v", fin)
	return fin
}

func (s *SenderState) SetErr(err error) {
	s.mut.Lock()
	s.exitErr = err
	s.mut.Unlock()
}

func (s *SenderState) GetErr() (err error) {
	s.mut.Lock()
	err = s.exitErr
	s.mut.Unlock()
	return
}