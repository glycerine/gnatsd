package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/glycerine/hnatsd/swp"
)

func main() {

	host := os.Getenv("BROKER_HOST")
	port := os.Getenv("BROKER_PORT")
	if host == "" {
		fmt.Fprintf(os.Stderr, "BROKER_HOST in env was not set. Setting required.\n")
		os.Exit(1)
	}
	if port == "" {
		fmt.Fprintf(os.Stderr, "BROKER_PORT in env was not set. Setting required.\n")
		os.Exit(1)
	}
	nport, err := strconv.Atoi(port)
	panicOn(err)

	fmt.Fprintf(os.Stderr, "contacting nats://%v:%v\n", host, port)

	// ===============================
	// setup nats client for a subscriber
	// ===============================

	subC := swp.NewNatsClientConfig(host, nport, "B", "B", true, false)
	sub := swp.NewNatsClient(subC)
	err = sub.Start()
	panicOn(err)
	defer sub.Close()

	// ===============================
	// make a session for each
	// ===============================
	var bnet *swp.NatsNet

	//fmt.Printf("sub = %#v\n", sub)

restart:
	for {
		if bnet != nil {
			bnet.Stop()
		}
		bnet = swp.NewNatsNet(sub)
		//fmt.Printf("recv.go is setting up NewSession...\n")
		to := time.Millisecond * 100
		B, err := swp.NewSession(swp.SessionConfig{Net: bnet, LocalInbox: "B", DestInbox: "A",
			WindowMsgCount: 1000, WindowByteSz: -1, Timeout: to, Clk: swp.RealClk,
			NumFailedKeepAlivesBeforeClosing: -1,
		})
		panicOn(err)

		//rep := swp.ReportOnSubscription(sub.Scrip)
		//fmt.Printf("rep = %#v\n", rep)

		msgLimit := int64(1000)
		bytesLimit := int64(600000)
		B.Swp.Sender.FlowCt = &swp.FlowCtrl{Flow: swp.Flow{
			ReservedByteCap: 600000,
			ReservedMsgCap:  1000,
		}}
		swp.SetSubscriptionLimits(sub.Scrip, msgLimit, bytesLimit)

		var n, ntot int64
		for {
			//fmt.Printf("\n ... about to receive on B.ReadMessagesCh %p\n", B.ReadMessagesCh)
			select {
			case seq := <-B.ReadMessagesCh:
				//fmt.Fprintf(os.Stderr, "\n recv.go got sequence len %v from B.ReadMessagesCh\n", len(seq.Seq))
				for _, pk := range seq.Seq {
					// copy to Stdout, handling short writes only.
					var from int64
					for {
						n, err = io.Copy(os.Stdout, bytes.NewBuffer(pk.Data[from:]))
						ntot += n
						if err == io.ErrShortWrite {
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
				//fmt.Printf("recv got B.Done\n")
				continue restart
			}
		}
	}
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
