package swp

import (
	"math"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test011RTTfromPackets(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := time.Hour

	var simClk = &SimClock{}
	t0 := time.Now()
	t1 := t0.Add(time.Second)
	t2 := t1.Add(time.Second)
	simClk.Set(t0)

	A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: simClk})
	panicOn(err)
	B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
		WindowMsgCount: 3, WindowByteSz: -1, Timeout: rtt, Clk: simClk})
	panicOn(err)

	A.IncrementClockOnReceiveForTesting()
	B.IncrementClockOnReceiveForTesting()

	p1 := &Packet{
		From:     "A",
		Dest:     "B",
		Data:     []byte("one"),
		TcpEvent: EventData,
	}

	var ack1 *Packet
	var rcTm time.Time
	gotAck := make(chan struct{})
	A.setPacketRecvCallback(func(pack *Packet) {
		p("A packet receive callback happened!")
		if ack1 == nil {
			ack1 = pack
			rcTm = simClk.Now()
			close(gotAck)
		}
	})

	A.Push(p1)

	r1 := <-B.ReadMessagesCh
	<-gotAck
	p("r1 = %#v", r1.Seq[0])

	p("ack1 = %#v", ack1)
	p("rcTm = %#v", rcTm)
	p("rcTm - t0 = %v", rcTm.Sub(t0))
	rtt1 := rcTm.Sub(ack1.DataSendTm)
	cv.Convey("Given 1 second clock steps at each receive, when A sends packet p1 to B at t0, arriving at t1, and the ack is returned at t2; then: the RTT computed should be t2-t0 or 2 seconds", t, func() {
		cv.So(rcTm, cv.ShouldResemble, t2)
		cv.So(rtt1, cv.ShouldResemble, 2*time.Second)

		p("Even if there is a retry, the Ack packet should be self contained, allowing RTT sampling from the DataSendTm that is updated on each retry")
		cv.So(ack1.ArrivedAtDestTm.Sub(ack1.DataSendTm), cv.ShouldResemble, rtt1)
	})

	A.Stop()
	B.Stop()

}

func Test012RTTestimation(t *testing.T) {
	samples := []int64{93, 8, 12, 41, 73, 32, 0, 71, 22, 84, 89, 130, 60, 39, 19, 183, 6, 117, 2, 166, 50, 60, 25, 20, 22, 67, 103, 42, 22, 96, 27, 22, 60, 84, 7, 10, 18, 248, 60, 33, 143, 55, 14, 183, 94, 67, 42, 9, 262, 279, 232, 127, 73, 30, 32, 172, 65, 29, 80, 258, 45, 26, 147, 89, 3, 166, 101, 152, 103, 39, 91, 26, 44, 282, 51, 67, 24, 127, 30, 239, 202, 128, 16, 8, 131, 26, 115, 1, 44, 81, 152, 19, 94, 12, 266, 51, 115, 72, 19, 128}

	expected := []float64{93.00000, 84.50000, 77.25000, 73.62500, 73.56250, 69.40625, 62.46563, 63.31906, 59.18716, 61.66844, 64.40160, 70.96144}

	rtt := NewRTT()
	cv.Convey("Given alpha=0.1 and single exponential smoothing, our RTT algorithm should produce values matching those manually computed in R", t, func() {
		for i := range expected {
			ex := expected[i]
			rtt.AddSample(time.Duration(samples[i] * 1e9))
			obs := rtt.GetEstimate()
			diff := math.Abs(float64(obs) - ex*1e9)

			//p("obs = %v, ex = %v, diff=%v", float64(int64(obs)), ex*1e9, diff)
			cv.So(diff, cv.ShouldBeLessThan, 1e6) // 1e6 => 3 decimal places must agree
		}
	})
}
