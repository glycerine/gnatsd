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

func Test108Compact(t *testing.T) {

	cv.Convey("Peer LocalSet should also Compact if we Set enough, and numSetsBeforeCompact is small, e.g. numSetsBeforeCompact == 1, and we should read the key we just set after the compaction.", t, func() {

		nPeerPort0, lsn0 := getAvailPort()
		nPeerPort2, lsn2 := getAvailPort()
		lsn0.Close()
		lsn2.Close()

		cluster0 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort2)

		// want peer0 to be lead, so we give it lower rank.
		peer0cfg := strings.Join([]string{"-rank=0", "-health", "-p", fmt.Sprintf("%v", nPeerPort0), cluster0}, " ")

		p0, err := NewPeer(peer0cfg, "p0", 1)
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
