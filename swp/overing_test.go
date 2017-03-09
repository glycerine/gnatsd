package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test014OverflowCheck(t *testing.T) {

	cv.Convey(`Given a sequence of events entering an EventRingBuf, if we see more N or more (failure to ack) events in Window time, then AddEventCheckOverflow should return true`, t, func() {
		maxEventsStored := 4
		maxTimeWindow := 2700 * time.Millisecond
		r := NewEventRingBuf(maxEventsStored, maxTimeWindow)

		t0 := time.Now()
		t1 := t0.Add(time.Second)
		t2 := t1.Add(time.Second)
		t3 := t2.Add(time.Second)             // [0,1,2,3] -> 4 events in 3 seconds, overflow is false
		t4 := t3.Add(500 * time.Millisecond)  // [3.5, 1,2,3] -> 4 or more events within 4 seconds (2.5 observed), overflow is true
		t5 := t4.Add(500 * time.Millisecond)  // [3.5, 4, 2, 3] -> 4 events in 2 seconds, overflow is true
		t6 := t0.Add(10 * time.Second)        // [3.5, 4, 10, 3] -> 4 events in 7 seconds, overflow is false
		t7 := t6.Add(500 * time.Millisecond)  // [3.5, 4, 10, 10.5] -> 4 events in 6.5 seconds, false
		t8 := t7.Add(500 * time.Millisecond)  // [11, 4, 10, 10.5] -> 4 events in 7 seconds, false
		t9 := t8.Add(1700 * time.Millisecond) // [11, 12.7, 10, 10.5] -> 4 events in 2.7 seconds, true

		cv.So(r.AddEventCheckOverflow(t0), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t1), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t2), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t3), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t4), cv.ShouldEqual, true)
		cv.So(r.AddEventCheckOverflow(t5), cv.ShouldEqual, true)
		cv.So(r.AddEventCheckOverflow(t6), cv.ShouldEqual, false)

		cv.So(r.AddEventCheckOverflow(t7), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t8), cv.ShouldEqual, false)
		cv.So(r.AddEventCheckOverflow(t9), cv.ShouldEqual, true)

	})
}
