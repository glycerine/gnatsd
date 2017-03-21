package swp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"github.com/glycerine/hnatsd/server"
	"github.com/glycerine/nats"
)

func Test040FileTransfer(t *testing.T) {

	cv.Convey("Big file transfer should succeed.", t, func() {

		// ===============================
		// begin generic nats setup
		// ===============================

		host := "127.0.0.1"
		port := getAvailPort()
		gnats := StartGnatsd(host, port)
		p("started gnatsd on %v:%v", host, port)
		defer func() {
			p("calling gnats.Shutdown()")
			gnats.Shutdown() // when done
		}()

		n := 1 << 24 // test with 16MB file.
		writeme := SequentialPayload(int64(n))

		p("writeme is")
		showSeq(writeme, 100000)
		//showSeq(writeme, 10)

		var buf bytes.Buffer
		recDone := make(chan bool)
		go testrec(host, port, gnats, &buf, recDone)
		testsender(host, port, gnats, writeme)
		<-recDone
		p("bytes transfered %v", len(buf.Bytes()))
		got := buf.Bytes()

		p("got is")
		showSeq(got, 100000)
		//showSeq(got, 10)

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
	})

}

func testsender(host string, nport int, gnats *server.Server, writeme []byte) {

	// ===============================
	// setup nats client for a publisher
	// ===============================

	skipTLS := true
	asyncErrCrash := false
	pubC := NewNatsClientConfig(host, nport, "A", "A", skipTLS, asyncErrCrash, nil)
	pub := NewNatsClient(pubC)
	err := pub.Start()
	panicOn(err)
	defer pub.Close() // this will shut down nats client too quickly.

	// ===============================
	// make a session for each
	// ===============================

	anet := NewNatsNet(pub)

	//fmt.Printf("pub = %#v\n", pub)

	to := time.Millisecond * 100
	windowby := int64(1 << 20)
	//windowby := int64(10)
	A, err := NewSession(SessionConfig{Net: anet, LocalInbox: "A", DestInbox: "B",
		WindowMsgCount: 1000, WindowByteSz: windowby, Timeout: to, Clk: RealClk})
	panicOn(err)

	//rep := ReportOnSubscription(pub.Scrip)
	//fmt.Printf("rep = %#v\n", rep)

	msgLimit := int64(1000)
	bytesLimit := int64(600000)
	//bytesLimit := int64(10)
	A.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap: 600000,
		ReservedMsgCap:  1000,
	}}
	SetSubscriptionLimits(pub.Scrip, msgLimit, bytesLimit)

	// writer does:
	/*
		        buf := make([]byte, 1<<20)
				// copy stdin over the wire
				for {
					_, err = io.CopyBuffer(A, os.Stdin, buf)
					if err == io.ErrShortWrite {
						continue
					} else {
						break
					}
				}
				//panicOn(err)
	*/

	//	by, err := ioutil.ReadAll(os.Stdin)
	//	panicOn(err)
	//	fmt.Fprintf(os.Stderr, "read %v bytes from stdin\n", len(by))

	A.simulateLostSynCount = 4
	n, err := A.Write(writeme)
	fmt.Fprintf(os.Stderr, "n = %v, err=%v after A.Write(writeme), where len(writeme)=%v\n", n, err, len(writeme))
	A.Close()
}

func testrec(host string, nport int, gnats *server.Server, dest io.Writer, done chan bool) {

	// ===============================
	// setup nats client for a subscriber
	// ===============================

	subC := NewNatsClientConfig(host, nport, "B", "B", true, false, nil)
	sub := NewNatsClient(subC)
	err := sub.Start()
	panicOn(err)
	defer sub.Close()

	// ===============================
	// make a session for each
	// ===============================
	var bnet *NatsNet

	//fmt.Printf("sub = %#v\n", sub)

	for {
		if bnet != nil {
			bnet.Stop()
		}
		bnet = NewNatsNet(sub)
		//fmt.Printf("recv.go is setting up NewSession...\n")
		to := time.Millisecond * 100
		B, err := NewSession(SessionConfig{Net: bnet, LocalInbox: "B", DestInbox: "A",
			WindowMsgCount: 1000, WindowByteSz: -1, Timeout: to, Clk: RealClk,
			NumFailedKeepAlivesBeforeClosing: -1,
		})
		panicOn(err)

		//rep := ReportOnSubscription(sub.Scrip)
		//fmt.Printf("rep = %#v\n", rep)

		msgLimit := int64(1000)
		bytesLimit := int64(600000)
		//bytesLimit := int64(10)
		B.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
			ReservedByteCap: 600000,
			ReservedMsgCap:  1000,
		}}
		SetSubscriptionLimits(sub.Scrip, msgLimit, bytesLimit)

		senderClosed := make(chan bool)
		B.Swp.Recver.AppCloseCallback = func() {
			//p("AppCloseCallback called. B.Swp.Recver.LastFrameClientConsumed=%v", B.Swp.Recver.LastFrameClientConsumed)
			close(senderClosed)
		}

		var n, ntot int64
		var expectedSeqNum int64
		for {
			//fmt.Printf("\n ... about to receive on B.ReadMessagesCh %p\n", B.ReadMessagesCh)
			select {
			case seq := <-B.ReadMessagesCh:
				//ns := len(seq.Seq)
				//fmt.Fprintf(os.Stderr, "\n B filetransfer_test testrec() got sequence len %v from B.ReadMessagesCh. SeqNum:[%v, %v]\n", ns, seq.Seq[0].SeqNum, seq.Seq[ns-1].SeqNum)
				for k, pk := range seq.Seq {
					if pk.SeqNum != expectedSeqNum {
						panic(fmt.Sprintf(
							"expected SeqNum %v, but got %v",
							expectedSeqNum, pk.SeqNum))
					}
					expectedSeqNum++
					// copy to dest, handling short writes only.
					var from int64
					for {
						n, err = io.Copy(dest, bytes.NewBuffer(pk.Data[from:]))
						_ = k
						//fmt.Fprintf(os.Stderr, "\n %v-th io.Copy gave n=%v, err=%v\n", k, n, err)
						ntot += n
						if err == io.ErrShortWrite {
							p("hanlding io.ErrShortWrite in copy loop")
							from += n
							continue
						} else {
							break
						}
					}
					panicOn(err)
					//fmt.Printf("\n")
					//fmt.Fprintf(os.Stderr, "\ndone with latest io.Copy, err was nil. n=%v, ntot=%v\n", n, ntot)
				}
			case <-B.Halt.Done.Chan:
				fmt.Printf("recv got B.Done\n")
				close(done)
				return

			case <-senderClosed:
				fmt.Printf("recv got senderClosed\n")
				close(done)
				return

				// ridiculous end-of-transfer indicator, but for debugging...
				//			case <-time.After(4 * time.Second):
				//				fmt.Printf("debug: recv loop timeout after 4 sec\n")
				//				close(done)
				//				return
			}
		}
	}
}

func SequentialPayload(n int64) []byte {
	if n%8 != 0 {
		panic(fmt.Sprintf("n == %v must be a multiple of 8; has remainder %v", n, n%8))
	}

	k := uint64(n / 8)
	by := make([]byte, n)
	j := uint64(0)
	for i := uint64(0); i < k; i++ {
		j = i * 8
		binary.LittleEndian.PutUint64(by[j:j+8], j)
	}
	return by
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

func showSeq(by []byte, m int) {
	//fmt.Printf("showSeq called with len(by)=%v, m=%v\n", len(by), m)
	fmt.Printf("\n")
	n := len(by)
	if n%8 != 0 {
		panic(fmt.Sprintf("len(by) == n == %v must be a multiple of 8; has remainder %v", n, n%8))
	}
	for i := 0; i*8+8 <= n; i = i + m {
		j := i * 8
		//p("i = %v.  j=%v. m=%v. n=%v. len(by)=%v.  (i+8)*8+8=%v <= n(%v) is %v", i, j, m, n, len(by), (i+m)*8+8, n, (i+m)*8+8 <= n)
		a := int64(binary.LittleEndian.Uint64(by[j : j+8]))
		fmt.Printf("at %08d: %08d\n", j, a)
		if a != int64(j) {
			panic(fmt.Sprintf("detected j != a, at j=%v, a=%v", int64(j), a))
		}
	}
}

func Test041File(t *testing.T) {

	cv.Convey("SequentialPayload() produces a byte-numbered octet payload", t, func() {
		for i := 8; i < 128; i += 8 {
			by := SequentialPayload(int64(i))
			showSeq(by, 1)
		}
	})
}

func Test042FileTransferWithReadAndWrite(t *testing.T) {

	cv.Convey("Big file transfer should succeed, when using Read and Write after our SetupRecvStream() and SetupSendStream() calls.", t, func() {

		// ===============================
		// begin generic nats setup
		// ===============================

		host := "127.0.0.1"
		port := getAvailPort()
		gnats := StartGnatsd(host, port)
		defer func() {
			p("calling gnats.Shutdown()")
			gnats.Shutdown() // when done
		}()

		n := 1 << 24
		writeme := SequentialPayload(int64(n))

		p("writeme is")
		showSeq(writeme, 100000)
		//showSeq(writeme, 10)

		recDone := make(chan bool)

		ignoreSlowConsumerErrors := true
		skipTLS := true
		//nc *nats.Conn,
		serverHost := host
		serverNport := port
		clientNameA := "A"
		clientNameB := "B"
		Asubj := "A"
		Bsubj := "B"

		ncA := mustNatsConn(serverHost, serverNport)
		sessA, err := SetupSendStream(
			ncA,
			serverHost,
			serverNport,
			clientNameA,
			Asubj,
			Bsubj,
			skipTLS,
			nil,
			ignoreSlowConsumerErrors)
		panicOn(err)

		ncB := mustNatsConn(serverHost, serverNport)
		sessB, err := SetupRecvStream(
			ncB,
			serverHost,
			serverNport,
			clientNameB,
			Bsubj,
			Asubj,
			skipTLS,
			nil,
			ignoreSlowConsumerErrors)
		panicOn(err)

		by := make([]byte, n)
		totnr := 0
		readAttempt := 0
		go func() {
			for totnr < n {
				//p("by=%p; len(by)=%v; totnr=%v", by, len(by), totnr)
				//p("by=%p; len(by)=%v; len(by[totnr:] is %v. totnr=%v", by, len(by), len(by[totnr:]), totnr)
				nr, err := sessB.Read(by[totnr:])
				panicOn(err)
				//p("nr = %v, on readAttempt=%v", nr, readAttempt)
				totnr += nr
				readAttempt++
			}
			close(recDone)
		}()
		nw, err := sessA.Write(writeme)
		panicOn(err)
		p("nw = %v", nw)
		<-recDone

		//		p("bytes transfered %v", len(buf.Bytes()))
		//		got := buf.Bytes()
		got := by

		p("got is")
		showSeq(got, 100000)
		//showSeq(got, 10)

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
	})

}

func mustNatsConn(serverHost string, serverPort int) *nats.Conn {

	natsURL := fmt.Sprintf("nats://%v:%v", serverHost, serverPort)
	recon := nats.MaxReconnects(-1) // retry forevever.
	norand := nats.DontRandomize()

	opts := []nats.Option{recon, norand}
	var nc *nats.Conn
	var err error
	try := 0
	tryLimit := 20

	for {
		nc, err = nats.Connect(natsURL, opts...)
		if err == nil {
			break
		}
		if try < tryLimit {
			p("nats.Connect() failed at try %v, with err '%v'. trying again after 1 second.", try, err)
			time.Sleep(time.Second)
			continue
		}
		if err != nil {
			msg := fmt.Errorf("Can't connect to "+
				"nats on url '%s': %v",
				natsURL,
				err)
			panic(msg)
		}
	}
	return nc
}

func Test043SendFileHeadersAheadOfCheckpoints(t *testing.T) {

	cv.Convey("For each checkpoint (big file), send each peer the file header which tells it the length and checksum up front, so they know when to stop reading.", t, func() {

		// ===============================
		// begin generic nats setup
		// ===============================

		host := "127.0.0.1"
		port := getAvailPort()
		gnats := StartGnatsd(host, port)
		defer func() {
			p("calling gnats.Shutdown()")
			gnats.Shutdown() // when done
		}()

		n := 1 << 25
		writeme := SequentialPayload(int64(n))

		p("writeme is")
		showSeq(writeme, 100000)
		//showSeq(writeme, 10)

		recDone := make(chan bool)

		ignoreSlowConsumerErrors := true
		skipTLS := true
		//nc *nats.Conn,
		serverHost := host
		serverNport := port
		clientNameA := "A"
		clientNameB := "B"
		Asubj := "A"
		Bsubj := "B"

		ncA := mustNatsConn(serverHost, serverNport)
		sessA, err := SetupSendStream(
			ncA,
			serverHost,
			serverNport,
			clientNameA,
			Asubj,
			Bsubj,
			skipTLS,
			nil,
			ignoreSlowConsumerErrors)
		panicOn(err)

		ncB := mustNatsConn(serverHost, serverNport)
		sessB, err := SetupRecvStream(
			ncB,
			serverHost,
			serverNport,
			clientNameB,
			Bsubj,
			Asubj,
			skipTLS,
			nil,
			ignoreSlowConsumerErrors)
		panicOn(err)

		var by []byte
		var sbf, rbf *BigFile
		go func() {
			var err error
			time.Sleep(time.Second)
			rbf, err = sessB.RecvFile()
			panicOn(err)
			by = rbf.Data
			close(recDone)
		}()

		path := "filetransfer.test.file"
		sbf, err = sessA.SendFile(path, writeme, time.Now())
		<-recDone

		//		p("bytes transfered %v", len(buf.Bytes()))
		//		got := buf.Bytes()
		got := by

		// verify checksum
		cksum := Blake2bOfBytes(rbf.Data)
		p("compare checksums, observed:'%x', expected:'%x'", rbf.Blake2b, sbf.Blake2b)
		cv.So(rbf.Blake2b, cv.ShouldResemble, cksum)
		cv.So(sbf.Blake2b, cv.ShouldResemble, cksum)

		p("got is")
		showSeq(got, 100000)
		//showSeq(got, 10)

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
	})

}
