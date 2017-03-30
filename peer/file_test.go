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

		p0, p1, p2, _ := testSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer cleanupTestUserDatabases()

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

/*
func Test105GetLatest(t *testing.T) {

	cv.Convey("Given three peers p0, p1, and p2, GetLatest should retreive the data with the most recent timestamp", t, func() {

		nPeerPort0, lsn0 := getAvailPort()
		nPeerPort1, lsn1 := getAvailPort()
		nPeerPort2, lsn2 := getAvailPort()
		nPeerPort3, lsn3 := getAvailPort()
		nPeerPort4, lsn4 := getAvailPort()
		nPeerPort5, lsn5 := getAvailPort()

		// don't close until now. Now we have non-overlapping ports.
		lsn0.Close()
		lsn1.Close()
		lsn2.Close()
		lsn3.Close()
		lsn4.Close()
		lsn5.Close()

		cluster0 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort2)
		cluster1 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort3)
		cluster2 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort4)
		routes1 := fmt.Sprintf("-routes=nats://localhost:%v", nPeerPort2)

		// want peer0 to be lead, so we give it lower rank.
		peer0cfg := strings.Join([]string{"-rank=0", "-health", "-p", fmt.Sprintf("%v", nPeerPort0), cluster0}, " ")

		peer1cfg := strings.Join([]string{"-rank=3", "-health", "-p", fmt.Sprintf("%v", nPeerPort1), cluster1, routes1}, " ")

		peer2cfg := strings.Join([]string{"-rank=6", "-health", "-p", fmt.Sprintf("%v", nPeerPort5), cluster2, routes1}, " ")

		p0, err := NewPeer(peer0cfg, "p0")
		panicOn(err)
		p1, err := NewPeer(peer1cfg, "p1")
		panicOn(err)
		p2, err := NewPeer(peer2cfg, "p2")
		panicOn(err)

		// start em up
		err = p0.Start()
		panicOn(err)
		err = p1.Start()
		panicOn(err)
		err = p2.Start()
		panicOn(err)

		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()

		// let peers come up and start talking
		peers, err := p0.WaitForPeerCount(3, 120*time.Second)
		p("peers = %#v", peers)
		if err != nil {
			panic(fmt.Sprintf("could not setup all 3 peers?!?: err = '%v'. ", err))
		}
		if len(peers.Members) != 3 {
			panic(fmt.Sprintf("could not setup all 3 peers??? count=%v. ", len(peers.Members)))
		}

		defer os.Remove("p0.boltdb")
		defer os.Remove("p1.boltdb")
		defer os.Remove("p2.boltdb")

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v plus blah", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v plush blahblah", t2))

		key := []byte("chk")

		err = p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&api.KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&api.KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		ki0, err := p0.GetLatest(key, true)

		cv.So(ki0.Key, cv.ShouldResemble, key)
		cv.So(ki0.When, cv.ShouldResemble, t2)
		cv.So(ki0.Size, cv.ShouldEqual, len(data2))
		cv.So(ki0.Val, cv.ShouldResemble, data2)
	})
}

*/

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
