package swp

// huge semaphore thrashing test: 17 seconds to finish.
/*

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test108ProvidesFlowControlToThrottleOverSending(t *testing.T) {

	f, err := os.Create("cpuprofile")
	if err != nil {
		mylog.Fatal(err)
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

	subC := NewNatsClientConfig(host, port, "B", "B", true, false)
	//subC.AsyncErrPanics = true
	sub := NewNatsClient(subC)
	err = sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A", "A", true, false)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
	defer pub.Close()

	// ===============================
	// make a session for each
	// ===============================

	anet := NewNatsNet(pub)
	bnet := NewNatsNet(sub)

	q("sub = %#v", sub)
	q("pub = %#v", pub)

	//lossProb := float64(0)
	lat := 1 * time.Millisecond

	rtt := 2 * lat

	p("Sender can send 10 at once, but receiver only wants 1 at a time")
	A, err := NewSession(anet, "A", "B", 1, -1, rtt, RealClk)
	panicOn(err)
	B, err := NewSession(bnet, "B", "A", 1, -1, rtt, RealClk)
	B.Swp.Sender.LastFrameSent = 999
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
	msgLimit := 100
	bytesLimit := 500000
	B.Swp.Sender.FlowCt = &FlowCtrl{flow: Flow{
		ReservedByteCap: 500000,
		ReservedMsgCap:  100,
	}}
	SetSubscriptionLimits(sub.Scrip, msgLimit, bytesLimit)

	// ===============================
	// setup publisher to produce
	// ===============================

	n := 100
	seq := make([]*Packet, n)
	for i := range seq {
		pack := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte(fmt.Sprintf("%v", i)),
			TcpEvent: EventData,
		}
		seq[i] = pack
	}

	readsAllDone := make(chan struct{})
	go func() {
		seen := 0
		for seen < n {
			//time.Sleep(500 * time.Millisecond)
			ios := <-B.ReadMessagesCh
			got := len(ios.Seq)
			seen += got
			if got > 0 {
				p("go read total of %v, the last is %v", seen, ios.Seq[got-1].SeqNum)
			} else {
				p("got 0 ?? : %#v", ios.Seq)
			}
		}
		p("done with all reads")
		close(readsAllDone)
	}()

	for i := range seq {
		A.Push(seq[i])
		p("push i=%v done", i)
	}
	<-readsAllDone

	A.Stop()
	B.Stop()

	// NOT DONE, WORK IN PROGRESS
	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers. The current test simply keeps a window of 3 messages on both sender and receiver, and runs 100 messages across the nats bus, checking that they all arrived at the end.", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}
*/
