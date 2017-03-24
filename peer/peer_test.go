package peer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

func Test001PeerToPeerFileTransfer(t *testing.T) {

	cv.Convey("Peers get and set key/value pairs between themselves. Where BcastKeyVal() will broadcast the change to all peers, and LocalGetKeyVal will locally query for the latest observed value for the key.", t, func() {

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

		p0, err := NewPeer(peer0cfg)
		panicOn(err)
		p1, err := NewPeer(peer1cfg)
		panicOn(err)
		p2, err := NewPeer(peer2cfg)
		panicOn(err)

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v", t2))
		data3 := []byte(fmt.Sprintf("dataset 3 at %v", t3))

		key := []byte("checkpoint")

		err = p0.StoreLocalKeyVal(key, data0, t0)
		panicOn(err)
		err = p1.StoreLocalKeyVal(key, data1, t1)
		panicOn(err)
		err = p2.StoreLocalKeyVal(key, data2, t2)
		panicOn(err)

		// GetLatest should return only the most
		// recent key, no matter where we query from.
		{
			dlatest, tmStampLatest, err := p0.GetLatest(key)
			panicOn(err)
			cv.So(tmStampLatest, cv.ShouldEqual, t2)
			cv.So(dlatest, cv.ShouldResemble, data2)

			// likewise, BcastGetKeyTimes, used by GetLatest,
			// should reveal who has what and when, without
			// doing full data value transfers. And the keys
			// should be sorted by increasing time.
			inv, err := p0.BcastGetKeyTimes(key)
			panicOn(err)
			cv.So(inv[0].Key, cv.ShouldResemble, key)
			cv.So(inv[0].When, cv.ShouldEqual, t0)
			cv.So(inv[1].Key, cv.ShouldResemble, key)
			cv.So(inv[1].When, cv.ShouldEqual, t1)
			cv.So(inv[2].Key, cv.ShouldResemble, key)
			cv.So(inv[2].When, cv.ShouldEqual, t2)
		}
		{
			dlatest, tmStampLatest, err := p1.GetLatest(key)
			panicOn(err)
			cv.So(tmStampLatest, cv.ShouldEqual, t2)
			cv.So(dlatest, cv.ShouldResemble, data2)
		}
		{
			dlatest, tmStampLatest, err := p2.GetLatest(key)
			panicOn(err)
			cv.So(tmStampLatest, cv.ShouldEqual, t2)
			cv.So(dlatest, cv.ShouldResemble, data2)
		}

		// BcastKeyValue should overwrite everywhere.
		err = p0.BcastKeyVal(key, data3, t3)
		panicOn(err)
		{
			got, gotTm, err := p0.GetLocal(key)
			panicOn(err)
			cv.So(gotTm, cv.ShouldEqual, t3)
			cv.So(got, cv.ShouldResemble, data3)
		}
		{
			got, gotTm, err := p1.GetLocal(key)
			panicOn(err)
			cv.So(gotTm, cv.ShouldEqual, t3)
			cv.So(got, cv.ShouldResemble, data3)
		}
		{
			got, gotTm, err := p2.GetLocal(key)
			panicOn(err)
			cv.So(gotTm, cv.ShouldEqual, t3)
			cv.So(got, cv.ShouldResemble, data3)
		}
	})
}
