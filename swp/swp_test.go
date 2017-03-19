package swp

import (
	"fmt"
	"sync/atomic"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

// check for:
//
// packet loss - eventually received and/or re-requested.
// duplicate receives - discard the 2nd one.
// out of order receives - reorder correctly.
//
// And flow control - client consumer buffers never overflow.
//

func Test001Network(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 2 * lat

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B", WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A", WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	p1 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("one"),
		TcpEvent: EventData,
	}

	A.Push(p1)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, sending a packet on a non-lossy network from A to B, the packet should arrive at B", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 1)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 1)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test002LostPacketTimesOutAndIsRetransmitted(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	net.DiscardOnce = 1
	rtt := 2 * lat

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B", WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A", WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	p1 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("one"),
		TcpEvent: EventData,
	}

	p2 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("two"),
		TcpEvent: EventData,
	}

	A.Push(p1)
	A.Push(p2)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, if a packet from A to B is lost, the timeout mechanism in the sender should notice that it didn't get the ack, and should resend.", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 2)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 2)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test003MisorderedPacketsAreReordered(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 2 * lat

	p("effectively turning off replays for this test")
	rtt = time.Hour
	// windowMsgSz must be at least two here in order for
	// the re-order to actually happen; else the test will
	// (and should) hang.
	windowMsgSz := int64(2)

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B", WindowMsgCount: windowMsgSz, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A", WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	// if LastSeenAvailReaderMsgCap is 1, then we never get
	// to send re-ordered 2nd packet, so make it at least 2.

	// this next line is done above now that we set buffers
	// from windowMsgSz and it is 2; so avoid this next
	// line, which is a data race:
	//A.Swp.Sender.LastSeenAvailReaderMsgCap = 2

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	p1 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("one"),
		TcpEvent: EventData,
	}

	p2 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("two"),
		TcpEvent: EventData,
	}

	net.SimulateReorderNext = 1

	A.Push(p1)

	time.Sleep(50 * time.Millisecond)

	p("due to the hold-back, B should not have gotten anything, packet should have been held.")
	if len(B.Swp.Recver.RecvHistory) != 0 {
		panic("should have some B history")
	}

	A.Push(p2)

	time.Sleep(300 * time.Millisecond)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, if a packets 0 and 1 from A to B are reordered so as to arrive in order 1, 0; the sliding window protocol should correctly re-order and deliver them in order.", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 2)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 2)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test004DuplicatedPacketIsDiscarded(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 2 * lat

	p("effectively turning off replays for this test")
	rtt = time.Hour
	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	p2 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("two"),
		TcpEvent: EventData,
	}

	p1 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("one"),
		TcpEvent: EventData,
	}

	A.Push(p1)
	atomic.StoreUint32(&net.DuplicateNext, 1)
	A.Push(p2)

	time.Sleep(100 * time.Millisecond)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, if the network (or resends) results in a packet with a duplicate SeqNum to one already received; the sliding window protocol should reject the delivery and drop the packet.", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 2)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 2)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test006AlgorithmWithstandsNoisyNetworks(t *testing.T) {

	// works quickly even with 20% packet loss:
	lossProb := float64(0.20)
	//	lossProb := float64(0.10)
	//	lossProb := float64(0.05)
	//  lossProb := float64(0)
	lat := 1 * time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 3 * lat

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	n := 100
	seq := make([]*Packet, n)
	for i := range seq {
		pack := &Packet{
			From:     "A",
			Dest:     "B",
			Data:     []byte(fmt.Sprintf("%v", i)),
			TcpEvent: EventData,
		}
		seq[i] = pack
	}

	for i := range seq {
		A.Push(seq[i])
	}

	time.Sleep(1000 * time.Millisecond)

	A.Stop()
	B.Stop()

	smy := net.Summary()
	smy.Print()

	cv.Convey("Given two nodes A and B, even if the network is noisy, the SWP should eventually deliver our sequence in order.", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test060NewSessionNonce(t *testing.T) {
	fmt.Printf("NewSessionNonce()= '%s'.\n", NewSessionNonce())
	fmt.Printf("NewSessionNonce()= '%s'.\n", NewSessionNonce())
	fmt.Printf("NewSessionNonce()= '%s'.\n", NewSessionNonce())
	fmt.Printf("NewSessionNonce()= '%s'.\n", NewSessionNonce())
	fmt.Printf("NewSessionNonce()= '%s'.\n", NewSessionNonce())
}
