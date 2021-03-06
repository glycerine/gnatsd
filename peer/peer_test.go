package peer

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/hnatsd/peer/api"

	cv "github.com/glycerine/goconvey/convey"
)

func Test101PeerToPeerKeyValueTransfer(t *testing.T) {

	cv.Convey("Peers get and set key/value pairs between themselves. Where BcastSet() will broadcast the change to all peers, and LocalGet will locally query for the latest observed value for the key. GetLatest() will survey all peers and return the most recent value.", t, func() {

		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v", t2))
		data3 := []byte(fmt.Sprintf("dataset 3 at %v", t3))

		key := []byte("chk")

		err := p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&api.KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&api.KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		// GetLatest should return only the most
		// recent key, no matter where we query from.
		{
			ki, err := p0.GetLatest(key, true)
			panicOn(err)
			cv.So(ki.When, cv.ShouldResemble, t2)
			cv.So(ki.Val, cv.ShouldResemble, data2)

			// likewise, BcastGetKeyTimes, used by GetLatest,
			// should reveal who has what and when, without
			// doing full data value transfers. And the keys
			// should be sorted by increasing time.
			to := time.Second * 10
			inv, err := p0.ClientInitiateBcastGet(key, false, to, "")
			panicOn(err)
			cv.So(inv[0].Key, cv.ShouldResemble, key)
			cv.So(inv[0].When, cv.ShouldResemble, t0)

			cv.So(inv[1].Key, cv.ShouldResemble, key)
			cv.So(inv[1].When, cv.ShouldResemble, t1)

			cv.So(inv[2].Key, cv.ShouldResemble, key)
			cv.So(inv[2].When, cv.ShouldResemble, t2)
		}
		{
			lat, err := p1.GetLatest(key, true)
			panicOn(err)
			cv.So(lat.When, cv.ShouldResemble, t2)
			cv.So(lat.Val, cv.ShouldResemble, data2)
		}
		{
			lat, err := p2.GetLatest(key, true)
			panicOn(err)
			cv.So(lat.When, cv.ShouldResemble, t2)
			cv.So(lat.Val, cv.ShouldResemble, data2)
		}

		// BcastKeyValue should overwrite everywhere.

		err = p0.BcastSet(&api.KeyInv{Key: key, Val: data3, When: t3})
		panicOn(err)
		{
			got, err := p0.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldResemble, t3)
			cv.So(got.Val, cv.ShouldResemble, data3)
		}
		{
			got, err := p1.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldResemble, t3)
			cv.So(got.Val, cv.ShouldResemble, data3)
		}
		{
			got, err := p2.LocalGet(key, true)
			panicOn(err)
			cv.So(got.When, cv.ShouldResemble, t3)
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

		p0, err := NewPeer(peer0cfg, "p0", 0)
		panicOn(err)
		defer os.Remove("p0.boltdb")

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		key := []byte("chk")

		err = p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0, Size: int64(len(data0))})
		panicOn(err)

		k, err := p0.LocalGet(key, true)
		//p("k = %#v", k)
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

		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v plus blah", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v plush blahblah", t2))

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
		inv, err := p0.ClientInitiateBcastGet(key, false, to, "")
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

		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v plus blah", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v plush blahblah", t2))
		data3 := []byte(fmt.Sprintf("dataset 3 at %v data3blah", t3))

		key := []byte("chk")

		err := p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
		panicOn(err)
		err = p1.LocalSet(&api.KeyInv{Key: key, Val: data1, When: t1})
		panicOn(err)
		err = p2.LocalSet(&api.KeyInv{Key: key, Val: data2, When: t2})
		panicOn(err)

		err = p0.BcastSet(&api.KeyInv{Key: key, Val: data3, When: t3})
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

		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v plus blah", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v plush blahblah", t2))

		key := []byte("chk")

		err := p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
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

func Test107GetLatestSkipEncryption(t *testing.T) {

	cv.Convey("Given three peers p0, p1, and p2, GetLatest should retreive the data with the most recent timestamp, even when no encyprtion is used (when running on a VPN that already provides encryption).", t, func() {

		p0, p1, p2, _ := UtilTestSetupThree(func(peer *Peer) {
			peer.SkipEncryption = true
		})
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		t3 := time.Now().UTC()
		t2 := t3.Add(-time.Minute)
		t1 := t2.Add(-time.Minute)
		t0 := t1.Add(-time.Minute)

		data0 := []byte(fmt.Sprintf("dataset 0 at %v", t0))
		data1 := []byte(fmt.Sprintf("dataset 1 at %v plus blah", t1))
		data2 := []byte(fmt.Sprintf("dataset 2 at %v plush blahblah", t2))

		key := []byte("chk")

		err := p0.LocalSet(&api.KeyInv{Key: key, Val: data0, When: t0})
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

/*
func Test106PeerGrpcAndInternalPortDiscovery(t *testing.T) {

	cv.Convey("StartPeriodicClusterAgentLocQueries() should result in our discovering the Internal and Grpc ports for each peer", t, func() {

		// peer.StartPeriodicClusterAgentLocQueries()
		p0, p1, p2, _ := UtilTestSetupThree(nil)
		defer p0.Stop()
		defer p1.Stop()
		defer p2.Stop()
		defer UtilCleanupTestUserDatabases()

		time.Sleep(10 * time.Minute)
	})
}
*/
