package swp

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test008ProvidesFlowControlToThrottleOverSending(t *testing.T) {

	f, err := os.Create("cpuprofile")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	// Given a consumer able to read at 1k messages/sec,
	// and a producer able to produce at 5k messages/sec,
	// we should see bandwidth across the network at the
	// rate at which the consumer allows via flow-control.
	// i.e.
	// consumer reads at a fixed 20% of the rate at which the
	// producer can produce, then we should see the producer
	// only sending at that 20% rate.
	//
	// implications:
	//
	// We should see the internal buffers
	// (in the receiving nats client library) staying
	// within range. We should never get an error from
	// nats saying that the buffers have overflowed and
	// messages have been dropped.

	// ===============================
	// begin generic nats setup
	// ===============================

	host := "127.0.0.1"
	port := getAvailPort()
	gnats := StartGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()

	// ===============================
	// setup nats clients for a publisher and a subscriber
	// ===============================

	subC := NewNatsClientConfig(host, port, "B", "B", true, false, nil)
	//subC.AsyncErrPanics = true
	sub := NewNatsClient(subC)
	err = sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A", "A", true, false, nil)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
	defer pub.Close()

	// ===============================
	// make a session for each
	// ===============================

	anet := NewNatsNet(pub)
	bnet := NewNatsNet(sub)

	//p("sub = %#v", sub)
	//p("pub = %#v", pub)

	//lossProb := float64(0)
	lat := 1 * time.Millisecond

	rtt := 100 * lat

	A, err := NewSession(SessionConfig{Net: anet, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 1, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	p("receiver only wants 1 at a time")
	// for some reason 1 at a time thrases the semaphores
	// somewhere in the Go runtime.
	B, err := NewSession(SessionConfig{Net: bnet, LocalInbox: "B", DestInbox: "A",
		WindowMsgCount: 1, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	// easier to reason about when manually debugging,
	// but is a data race:
	// B.Swp.Sender.LastFrameSent = 999

	panicOn(err)

	A.SelfConsumeForTesting()
	//B.SelfConsumeForTesting()

	// ===============================
	// setup subscriber to consume at 1 message/sec
	// ===============================

	rep := ReportOnSubscription(sub.Scrip)
	p("rep = %#v", rep)

	// this limit alone is the first test for flow
	// control, since with a 10 message limit we'll quickly
	// overflow the client-side nats internal
	// buffer, and panic since 	subC.AsyncErrPanics = true
	// when trying to send 100 messages in a row.
	msgLimit := int64(100)
	bytesLimit := int64(500000)
	B.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap: 500000,
		ReservedMsgCap:  100,
	}}
	SetSubscriptionLimits(sub.Scrip, msgLimit, bytesLimit)

	// ===============================
	// setup publisher to produce
	// ===============================

	n := 20
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

	pushCh := make([]chan struct{}, n)
	for i := range pushCh {
		pushCh[i] = make(chan struct{}, 0)
	}

	readsAllDone := make(chan struct{})

	p("verify flow-control: with 1 receive slot, we should see lockstep 1-by-1 ping-pong send/receives happening.")
	go func() {
		for i := 0; i < n; i++ {
			p("verify that push has be unable to race ahead;")
			p("hence the pushCh[i+1 = %v] should not have been closed yet.", i+1)
			if i < n-1 {
				select {
				case <-pushCh[i+1]:
					panic("sender got ahead of where it should be! -- did not run in lock step respecting the flow control sequence!")
				case <-time.After(10 * time.Millisecond):
					// okay, push didn't race ahead
				}
			}
			<-B.ReadMessagesCh
			p("receive done at i = %v", i)
			// this will have freed up the next push.
			<-pushCh[i] // make sure we get to push i, but not push i+1
		}
		p("done with all reads")
		close(readsAllDone)
	}()

	for i := range seq {
		A.Push(seq[i])
		p("push i=%v done", i)
		close(pushCh[i])
	}
	<-readsAllDone

	A.Stop()
	B.Stop()

	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers. The current test verifies that flow control was exerted and only one message at a time was sent before it being read by the consumer appliadtion.", t, func() {
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, n)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, n)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test009SimNetVerifiesFlowControlNotViolated(t *testing.T) {

	// Same as Test008 but use SimNet in order to verify
	// that our flow control properties are not violated:
	// that the sender only ever sends the number of
	// frames <= the advertised available amounts of
	// the receiver.

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 1000 * lat

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	B.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap:     64 * 1024,
		ReservedMsgCap:      10,
		AvailReaderBytesCap: 5000,
		AvailReaderMsgCap:   1,
	}}
	A.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap:     64 * 1024,
		ReservedMsgCap:      10,
		AvailReaderBytesCap: 5000,
		AvailReaderMsgCap:   1,
	}}
	// SimNet should automatically panic if there
	// is a flow control violation.

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

	// NOT DONE, WORK IN PROGRESS
	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers. This version of the test uses the SimNet network simulator, and currently is incomplete: it just runs 100 messages across the wire using a small window of size 3 on either end.", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}
