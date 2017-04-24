package peer

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"github.com/glycerine/hnatsd/peer/api"
)

func Test203BcastGetBigFile(t *testing.T) {

	cv.Convey("Given three peers p0, p1, and p2, BcastGet should return KeyInv from both", t, func() {

		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		//n := 1 << 24 // 16MB
		//n := 1 << 29 // 512MB -- failed with dropped packets and timeout
		n := 1 << 28 // 256MB succeeded.
		//n := 1 << 30 // 1GB --failed with slow consumer detected. never finished after 90 seconds. // hung at peer.go:203
		data0 := SequentialPayload(int64(n), 0)
		data1 := SequentialPayload(int64(n), 1)
		data2 := SequentialPayload(int64(n), 2)
		data3 := SequentialPayload(int64(n), 3)

		key := []byte("chk")

		err := p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&api.KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&api.KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		// likewise, BcastGetKeyTimes, used by GetLatest,
		// should reveal who has what and when, without
		// doing full data value transfers. And the keys
		// should be sorted by increasing time.
		to := time.Second * 60
		// fetching only metadata
		inv, err := p0.ClientInitiateBcastGet(key, false, to, "")
		panicOn(err)
		cv.So(len(inv), cv.ShouldEqual, 3)
		cv.So(inv[0].Key, cv.ShouldResemble, key)
		cv.So(inv[0].When, cv.ShouldResemble, t0)
		cv.So(inv[0].Size, cv.ShouldEqual, len(data0))
		cv.So(inv[0].Val, cv.ShouldBeNil)
		cv.So(inv[0].Blake2b, cv.ShouldResemble, blake2bOfBytes(data0))

		cv.So(inv[1].Key, cv.ShouldResemble, key)
		cv.So(inv[1].When, cv.ShouldResemble, t1)
		cv.So(inv[1].Size, cv.ShouldEqual, len(data1))
		cv.So(inv[1].Val, cv.ShouldBeNil)
		cv.So(inv[1].Blake2b, cv.ShouldResemble, blake2bOfBytes(data1))

		cv.So(inv[2].Key, cv.ShouldResemble, key)
		cv.So(inv[2].When, cv.ShouldResemble, t2)
		cv.So(inv[2].Size, cv.ShouldEqual, len(data2))
		cv.So(inv[2].Val, cv.ShouldBeNil)
		cv.So(inv[2].Blake2b, cv.ShouldResemble, blake2bOfBytes(data2))

		// verify full data on all local copies; with LocalGet.
		{
			ki0, err := p0.LocalGet(key, true)
			panicOn(err)
			cv.So(ki0.Key, cv.ShouldResemble, key)
			cv.So(ki0.When, cv.ShouldResemble, t0)
			cv.So(ki0.Size, cv.ShouldEqual, len(data0))
			cv.So(ki0.Val, cv.ShouldResemble, data0)
			cv.So(ki0.Blake2b, cv.ShouldResemble, blake2bOfBytes(data0))
			verifySame(data0, ki0.Val)

			ki1, err := p1.LocalGet(key, true)
			panicOn(err)
			cv.So(ki1.Key, cv.ShouldResemble, key)
			cv.So(ki1.When, cv.ShouldResemble, t1)
			cv.So(ki1.Size, cv.ShouldEqual, len(data1))
			cv.So(ki1.Val, cv.ShouldResemble, data1)
			cv.So(ki1.Blake2b, cv.ShouldResemble, blake2bOfBytes(data1))
			verifySame(data1, ki1.Val)

			ki2, err := p2.LocalGet(key, true)
			panicOn(err)
			cv.So(ki2.Key, cv.ShouldResemble, key)
			cv.So(ki2.When, cv.ShouldResemble, t2)
			cv.So(ki2.Size, cv.ShouldEqual, len(data2))
			cv.So(ki2.Val, cv.ShouldResemble, data2)
			cv.So(ki2.Blake2b, cv.ShouldResemble, blake2bOfBytes(data2))
			verifySame(data2, ki2.Val)
		}
		p("Good: LocalGet() returned fully validated storage for size = %v", len(data0))

		// now broadcast a big file

		err = p0.BcastSet(&api.KeyInv{Key: key, Val: data3, When: t3})
		panicOn(err)

		// verify all local copies.
		ki0, err := p0.LocalGet(key, true)
		panicOn(err)
		cv.So(ki0.Key, cv.ShouldResemble, key)
		cv.So(ki0.When, cv.ShouldResemble, t3)
		cv.So(ki0.Size, cv.ShouldEqual, len(data3))
		cv.So(ki0.Val, cv.ShouldResemble, data3)
		cv.So(ki0.Blake2b, cv.ShouldResemble, blake2bOfBytes(data3))
		verifySame(data3, ki0.Val)

		ki1, err := p1.LocalGet(key, true)
		panicOn(err)
		cv.So(ki1.Key, cv.ShouldResemble, key)
		cv.So(ki1.When, cv.ShouldResemble, t3)
		cv.So(ki1.Size, cv.ShouldEqual, len(data3))
		cv.So(ki1.Val, cv.ShouldResemble, data3)
		cv.So(ki1.Blake2b, cv.ShouldResemble, blake2bOfBytes(data3))
		verifySame(data3, ki1.Val)

		ki2, err := p2.LocalGet(key, true)
		panicOn(err)
		cv.So(ki2.Key, cv.ShouldResemble, key)
		cv.So(ki2.When, cv.ShouldResemble, t3)
		cv.So(ki2.Size, cv.ShouldEqual, len(data3))
		cv.So(ki2.Val, cv.ShouldResemble, data3)
		cv.So(ki2.Blake2b, cv.ShouldResemble, blake2bOfBytes(data3))
		verifySame(data3, ki2.Val)

	})
}

func SequentialPayload(n int64, offset uint64) []byte {
	if n%8 != 0 {
		panic(fmt.Sprintf("n == %v must be a multiple of 8; has remainder %v", n, n%8))
	}

	k := uint64(n / 8)
	by := make([]byte, n)
	j := uint64(0)
	for i := uint64(0); i < k; i++ {
		j = i * 8
		binary.LittleEndian.PutUint64(by[j:j+8], j+offset)
	}
	return by
}

func verifySame(writeme, got []byte) {
	cv.So(len(got), cv.ShouldResemble, len(writeme))
	firstDiff := -1
	for i := 0; i < len(got); i++ {
		if got[i] != writeme[i] {
			firstDiff = i
			break
		}
	}
	if firstDiff != -1 {
		p("first Diff at %v, got %v, expected %v", firstDiff, got[firstDiff], writeme[firstDiff])
		a, b, c := nearestOctet(firstDiff, got)
		wa, wb, wc := nearestOctet(firstDiff, writeme)
		p("first Diff at %v for got: [%v, %v, %v]; for writem: [%v, %v, %v]", firstDiff, a, b, c, wa, wb, wc)
	}
	cv.So(firstDiff, cv.ShouldResemble, -1)
}

func nearestOctet(pos int, by []byte) (a, b, c int64) {
	n := len(by)
	pos -= (pos % 8)
	if pos-8 >= 0 && pos < n {
		a = int64(binary.LittleEndian.Uint64(by[pos-8 : pos]))
	}
	if pos >= 0 && pos+8 < n {
		b = int64(binary.LittleEndian.Uint64(by[pos : pos+8]))
	}
	if pos+8 >= 0 && pos+16 < n {
		c = int64(binary.LittleEndian.Uint64(by[pos+8 : pos+16]))
	}
	return a, b, c
}
