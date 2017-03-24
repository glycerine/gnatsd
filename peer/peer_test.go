package peer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

func Test001PeerToPeerKeyFileTransfer(t *testing.T) {

	cv.Convey("Peers get and set key/value pairs between themselves. Where BcastSet() will broadcast the change to all peers, and LocalGet will locally query for the latest observed value for the key. GetLatest() will survey all peers and return the most recent value.", t, func() {

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

		defer os.Remove("p0.boltdb")
		defer os.Remove("p1.boltdb")
		defer os.Remove("p2.boltdb")

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v", t2))
		data3 := []byte(fmt.Sprintf("dataset 3 at %v", t3))

		key := []byte("chk")

		err = p0.LocalSet(&KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		// GetLatest should return only the most
		// recent key, no matter where we query from.
		{
			ki, err := p0.GetLatest(key, true)
			panicOn(err)
			cv.So(ki.When, cv.ShouldEqual, t2)
			cv.So(ki.Val, cv.ShouldResemble, data2)

			// likewise, BcastGetKeyTimes, used by GetLatest,
			// should reveal who has what and when, without
			// doing full data value transfers. And the keys
			// should be sorted by increasing time.
			to := time.Second * 10
			inv, err := p0.BcastGet(key, false, to, "")
			panicOn(err)
			cv.So(inv[0].Key, cv.ShouldResemble, key)
			cv.So(inv[0].When, cv.ShouldEqual, t0)

			cv.So(inv[1].Key, cv.ShouldResemble, key)
			cv.So(inv[1].When, cv.ShouldEqual, t1)

			cv.So(inv[2].Key, cv.ShouldResemble, key)
			cv.So(inv[2].When, cv.ShouldEqual, t2)
		}
		{
			lat, err := p1.GetLatest(key, true)
			panicOn(err)
			cv.So(lat.When, cv.ShouldEqual, t2)
			cv.So(lat.Val, cv.ShouldResemble, data2)
		}
		{
			lat, err := p2.GetLatest(key, true)
			panicOn(err)
			cv.So(lat.When, cv.ShouldEqual, t2)
			cv.So(lat.Val, cv.ShouldResemble, data2)
		}

		// BcastKeyValue should overwrite everywhere.

		err = p0.BcastSet(&KeyInv{Key: key, Val: data3, When: t3})
		panicOn(err)
		{
			got, err := p0.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldEqual, t3)
			cv.So(got.Val, cv.ShouldResemble, data3)
		}
		{
			got, err := p1.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldEqual, t3)
			cv.So(got.Val, cv.ShouldResemble, data3)
		}
		{
			got, err := p2.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldEqual, t3)
			cv.So(got.Val, cv.ShouldResemble, data3)
		}
	})
}

func Test102LocalSet(t *testing.T) {

	cv.Convey("Peer LocalSet should save the ki locally", t, func() {

		nPeerPort0, lsn0 := getAvailPort()
		nPeerPort2, lsn2 := getAvailPort()
		lsn0.Close()
		lsn2.Close()

		cluster0 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort2)

		// want peer0 to be lead, so we give it lower rank.
		peer0cfg := strings.Join([]string{"-rank=0", "-health", "-p", fmt.Sprintf("%v", nPeerPort0), cluster0}, " ")

		p0, err := NewPeer(peer0cfg, "p0")
		panicOn(err)
		defer os.Remove("p0.boltdb")

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		key := []byte("chk")

		err = p0.LocalSet(&KeyInv{Key: key, Val: data0, When: t0, Size: int64(len(data0))})
		panicOn(err)

		k, err := p0.LocalGet(key, true)
		p("k = %#v", k)
		panicOn(err)
		cv.So(string(k.Key), cv.ShouldResemble, string(key))
		cv.So(k.Key, cv.ShouldResemble, key)
		cv.So(k.Size, cv.ShouldResemble, int64(len(data0)))
		cv.So(k.Val, cv.ShouldResemble, data0)
		cv.So(k.When, cv.ShouldResemble, t0)
	})
}

func Test103BcastGet(t *testing.T) {

	cv.Convey("Given three peers p0, p1, and p2, BcastGet should return KeyInv from both", t, func() {

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
		if err != nil || peers == nil || len(peers.Members) != 3 {
			p("peers = %#v", peers)
			panic(fmt.Sprintf("could not setup all 3 peers?!?: err = '%v'. ", err))
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

		err = p0.LocalSet(&KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		// likewise, BcastGetKeyTimes, used by GetLatest,
		// should reveal who has what and when, without
		// doing full data value transfers. And the keys
		// should be sorted by increasing time.
		to := time.Second * 60
		inv, err := p0.BcastGet(key, false, to, "")
		panicOn(err)
		cv.So(len(inv), cv.ShouldEqual, 3)
		cv.So(inv[0].Key, cv.ShouldResemble, key)
		cv.So(inv[0].When, cv.ShouldResemble, t0)
		cv.So(inv[0].Size, cv.ShouldEqual, len(data0))
		cv.So(inv[0].Val, cv.ShouldBeNil)

		cv.So(inv[1].Key, cv.ShouldResemble, key)
		cv.So(inv[1].When, cv.ShouldResemble, t1)
		cv.So(inv[1].Size, cv.ShouldEqual, len(data1))
		cv.So(inv[1].Val, cv.ShouldBeNil)

		cv.So(inv[2].Key, cv.ShouldResemble, key)
		cv.So(inv[2].When, cv.ShouldResemble, t2)
		cv.So(inv[2].Size, cv.ShouldEqual, len(data2))
		cv.So(inv[2].Val, cv.ShouldBeNil)
	})
}

func Test104BcastSet(t *testing.T) {

	cv.Convey("Given three peers p0, p1, and p2, BcastSet should set the broadcast value on on all peers", t, func() {

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
		data3 := []byte(fmt.Sprintf("dataset 3 at %v data3blah", t3))

		key := []byte("chk")

		err = p0.LocalSet(&KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		err = p0.BcastSet(&KeyInv{Key: key, Val: data3, When: t3})
		panicOn(err)

		// verify all local copies.
		ki0, err := p0.LocalGet(key, true)
		panicOn(err)
		cv.So(ki0.Key, cv.ShouldResemble, key)
		cv.So(ki0.When, cv.ShouldResemble, t3)
		cv.So(ki0.Size, cv.ShouldEqual, len(data3))
		cv.So(ki0.Val, cv.ShouldResemble, data3)

		ki1, err := p1.LocalGet(key, true)
		panicOn(err)
		cv.So(ki1.Key, cv.ShouldResemble, key)
		cv.So(ki1.When, cv.ShouldResemble, t3)
		cv.So(ki1.Size, cv.ShouldEqual, len(data3))
		cv.So(ki1.Val, cv.ShouldResemble, data3)

		ki2, err := p2.LocalGet(key, true)
		panicOn(err)
		cv.So(ki2.Key, cv.ShouldResemble, key)
		cv.So(ki2.When, cv.ShouldResemble, t3)
		cv.So(ki2.Size, cv.ShouldEqual, len(data3))
		cv.So(ki2.Val, cv.ShouldResemble, data3)

	})
}

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

		err = p0.LocalSet(&KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		ki0, err := p0.GetLatest(key, true)

		cv.So(ki0.Key, cv.ShouldResemble, key)
		cv.So(ki0.When, cv.ShouldResemble, t2)
		cv.So(ki0.Size, cv.ShouldEqual, len(data2))
		cv.So(ki0.Val, cv.ShouldResemble, data2)
	})
}
