package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func TestRetreeDeleteSlotWorks700(t *testing.T) {

	cv.Convey("retree(compareRetryDeadline) should be able to delete by slot", t, func() {

		SentButNotAckedByDeadline := newRetree(compareRetryDeadline)
		tree := SentButNotAckedByDeadline

		now := time.Now()
		slot0 := &TxqSlot{
			RetryDeadline: now,
			Pack:          &Packet{SeqNum: 0},
		}
		slot1 := &TxqSlot{
			RetryDeadline: now.Add(time.Second),
			Pack:          &Packet{SeqNum: 1},
		}

		tree.insert(slot0)
		tree.insert(slot1)

		cv.So(tree.tree.Len(), cv.ShouldEqual, 2)

		tree.deleteSlot(slot0)
		cv.So(tree.tree.Len(), cv.ShouldEqual, 1)

		tree.deleteSlot(slot1)
		cv.So(tree.tree.Len(), cv.ShouldEqual, 0)

	})
}

func TestRetreeDeleteSlotWorks701(t *testing.T) {

	cv.Convey("retree(compareSeqNum) should be able to delete by slot", t, func() {

		SentButNotAckedBySeqNum := newRetree(compareSeqNum)
		tree := SentButNotAckedBySeqNum

		now := time.Now()
		slot0 := &TxqSlot{
			RetryDeadline: now,
			Pack:          &Packet{SeqNum: 0},
		}
		slot1 := &TxqSlot{
			RetryDeadline: now.Add(time.Second),
			Pack:          &Packet{SeqNum: 1},
		}

		tree.insert(slot0)
		tree.insert(slot1)

		cv.So(tree.tree.Len(), cv.ShouldEqual, 2)

		tree.deleteSlot(slot0)
		cv.So(tree.tree.Len(), cv.ShouldEqual, 1)

		tree.deleteSlot(slot1)
		cv.So(tree.tree.Len(), cv.ShouldEqual, 0)

	})
}

func TestRetreeDeleteThroughSeqNumWorks702(t *testing.T) {

	cv.Convey("retree(compareSeqNum) should be able to deleteThroughSeqNum", t, func() {

		SentButNotAckedBySeqNum := newRetree(compareSeqNum)
		SentButNotAckedByDeadline := newRetree(compareRetryDeadline)

		now := time.Now()
		slot0 := &TxqSlot{
			RetryDeadline: now,
			Pack:          &Packet{SeqNum: 0},
		}
		slot1 := &TxqSlot{
			RetryDeadline: now.Add(time.Second),
			Pack:          &Packet{SeqNum: 1},
		}

		SentButNotAckedBySeqNum.insert(slot0)
		SentButNotAckedByDeadline.insert(slot0)

		SentButNotAckedBySeqNum.insert(slot1)
		SentButNotAckedByDeadline.insert(slot1)

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 2)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 2)

		SentButNotAckedBySeqNum.deleteThroughSeqNum(
			2, func(slot *TxqSlot) {
				SentButNotAckedByDeadline.deleteSlot(slot)
			})

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 0)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 0)
	})
}

func TestRetreeDeleteThroughDeadlineWorks703(t *testing.T) {

	cv.Convey("retree(compareRetryDeadline) should be able to deleteThroughDeadline", t, func() {

		SentButNotAckedBySeqNum := newRetree(compareSeqNum)
		SentButNotAckedByDeadline := newRetree(compareRetryDeadline)

		t0 := time.Now()
		slot0 := &TxqSlot{
			RetryDeadline: t0,
			Pack:          &Packet{SeqNum: 0},
		}
		t1 := t0.Add(time.Second)
		slot1 := &TxqSlot{
			RetryDeadline: t1,
			Pack:          &Packet{SeqNum: 1},
		}

		SentButNotAckedBySeqNum.insert(slot0)
		SentButNotAckedByDeadline.insert(slot0)

		SentButNotAckedBySeqNum.insert(slot1)
		SentButNotAckedByDeadline.insert(slot1)

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 2)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 2)

		SentButNotAckedByDeadline.deleteThroughDeadline(
			t1, func(slot *TxqSlot) {
				SentButNotAckedBySeqNum.deleteSlot(slot)
			})

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 0)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 0)
	})
}

func TestRetreeDeleteThroughSeqNumWorks704(t *testing.T) {

	cv.Convey("retree(compareSeqNum) should be able to deleteThroughSeqNum", t, func() {

		SentButNotAckedBySeqNum := newRetree(compareSeqNum)
		SentButNotAckedByDeadline := newRetree(compareRetryDeadline)

		t0 := time.Now()
		t1 := t0.Add(time.Second)
		t2 := t1.Add(time.Second)

		slot0 := &TxqSlot{
			RetryDeadline: t0,
			Pack:          &Packet{SeqNum: 0},
		}
		slot1 := &TxqSlot{
			RetryDeadline: t1,
			Pack:          &Packet{SeqNum: 1},
		}
		slot2 := &TxqSlot{
			RetryDeadline: t2,
			Pack:          &Packet{SeqNum: 2},
		}

		SentButNotAckedBySeqNum.insert(slot0)
		SentButNotAckedByDeadline.insert(slot0)

		SentButNotAckedBySeqNum.insert(slot1)
		SentButNotAckedByDeadline.insert(slot1)

		SentButNotAckedBySeqNum.insert(slot2)
		SentButNotAckedByDeadline.insert(slot2)

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 3)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 3)

		var through int64
		numDel := 0
		SentButNotAckedBySeqNum.deleteThroughSeqNum(
			through, func(slot *TxqSlot) {
				SentButNotAckedByDeadline.deleteSlot(slot)
				numDel++
			})

		cv.So(SentButNotAckedBySeqNum.tree.Len(), cv.ShouldEqual, 2)
		cv.So(SentButNotAckedByDeadline.tree.Len(), cv.ShouldEqual, 2)

		p("after numDel %v through=%v, s.SentButNotAckedBySeqNum=\n%s\n, and s.SentButNotAckedByDeadline=\n%s\n",
			numDel, through,
			SentButNotAckedBySeqNum,
			SentButNotAckedByDeadline)

	})
}
