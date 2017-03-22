package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test015RetryOfSynSynAckAndFin(t *testing.T) {

	cv.Convey("if the syn or synack gets lost, we should retry it. We should be able to tolerate loss of estabAck packet as well.", t, func() {

		lossProb := float64(0)
		lat := time.Millisecond
		net := NewSimNet(lossProb, lat)
		synLost := 2
		net.FilterThisEvent[EventSyn] = &synLost
		synAckLost := 2
		net.FilterThisEvent[EventSynAck] = &synAckLost
		estabAckLost := 2
		net.FilterThisEvent[EventEstabAck] = &estabAckLost
		rtt := 2 * lat

		// EventEstabAck does not get retried.
		// This is a problem if the EventEstabAck packet is lost.
		// The client can be in Established, but the
		// server is stuck in SynReceived.
		//
		// Solution: we piggy back state on the keepalives that
		// are sent regularly in the established state.
		// Our tcp logic checks for these and the server
		// will go from SynReceived to Established
		// on getting notice from a keepalive that the
		// client has transitioned to Established.

		A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
			WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk,
			NumFailedKeepAlivesBeforeClosing: 20, KeepAliveInterval: time.Second,
		})
		panicOn(err)
		B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
			WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk,
			NumFailedKeepAlivesBeforeClosing: 20, KeepAliveInterval: time.Second,
		})
		_ = B
		panicOn(err)

		B.ConnectTimeout = time.Second
		B.ConnectAttempts = 10

		A.ConnectTimeout = time.Second
		A.ConnectAttempts = 10
		err = A.Connect("B")
		panicOn(err)

		select {
		case <-time.After(30 * time.Second):

			// neither side should halt prematurely
			// from timeout of no-contact.
		case <-A.Halt.Done.Chan:
			p("A halted")
			cv.So(false, cv.ShouldEqual, true)
		case <-B.Halt.Done.Chan:
			p("B halted")
			cv.So(false, cv.ShouldEqual, true)
		}

		// should get here
		cv.So(true, cv.ShouldEqual, true)
	})
}
