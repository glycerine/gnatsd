package swp

import (
	"fmt"
	"time"

	"github.com/glycerine/nats"
	//"github.com/glycerine/hnatsd/server"
)

var ErrSessDone = fmt.Errorf("got session Done")
var ErrSenderClosed = fmt.Errorf("got senderClosed")

// SetupRecvStream sets up to receives a stream on bytes
// over the existing nc, which should be a live
// *nats.Conn that is already connected to a
// server. We will the nc subscribe to listen on the
// mysubj topic.
//
// if errorCallbackFunc is nil, we will panic on nats errors,
// with one exception: we respect the ignoreSlowConsumerErrors flag.
// So if ignoreSlowConsumerErrors we won't panic on a slow
// consumer, even if errorCallbackFunc is nil.
//
// after SetupRecvStream(), just do
//     n, err := sessB.Read(p)
// with the sessB returned. This will
// read from the session stream, into a p []byte.
// This is the standard interface Reader and Read() method.
//
func SetupRecvStream(
	nc *nats.Conn,
	serverHost string,
	serverNport int,
	clientName string,
	mysubj string,
	fromsubj string,
	skipTLS bool,
	errorCallbackFunc func(c *nats.Conn, s *nats.Subscription, e error),
	ignoreSlowConsumerErrors bool,

) (*Session, error) {

	asyncErrCrash := false
	cfg := NewNatsClientConfig(serverHost, serverNport, clientName, mysubj, skipTLS, asyncErrCrash, errorCallbackFunc)
	cli := NewNatsClientAlreadyStarted(cfg, nc)

	nnet := NewNatsNet(cli)
	//fmt.Printf("recv.go is setting up NewSession...\n")
	to := time.Millisecond * 100
	sess, err := NewSession(SessionConfig{Net: nnet, LocalInbox: mysubj, DestInbox: fromsubj,
		WindowMsgCount: 1000, WindowByteSz: -1, Timeout: to, Clk: RealClk,
		NumFailedKeepAlivesBeforeClosing: -1,
	})
	if err != nil {
		return nil, err
	}

	//rep := ReportOnSubscription(sub.Scrip)
	//fmt.Printf("rep = %#v\n", rep)

	msgLimit := int64(1000)
	bytesLimit := int64(600000)
	sess.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap: 600000,
		ReservedMsgCap:  1000,
	}}
	SetSubscriptionLimits(cli.Scrip, msgLimit, bytesLimit)

	sess.Swp.Recver.AppCloseCallback = func() {
		//p("AppCloseCallback called. sess.Swp.Recver.LastFrameClientConsumed=%v", sess.Swp.Recver.LastFrameClientConsumed)
		close(sess.RemoteSenderClosed)
	}
	return sess, nil
}

// just do
//     n, err := sessA.Write(writeme)
// with the sessA returned.
//
func SetupSendStream(
	nc *nats.Conn,
	serverHost string,
	serverNport int,
	clientName string,
	mysubj string,
	fromsubj string,
	skipTLS bool,
	errorCallbackFunc func(c *nats.Conn, s *nats.Subscription, e error),
	ignoreSlowConsumerErrors bool,

) (*Session, error) {

	asyncErrCrash := false
	cfg := NewNatsClientConfig(serverHost, serverNport, clientName, mysubj, skipTLS, asyncErrCrash, errorCallbackFunc)
	cli := NewNatsClientAlreadyStarted(cfg, nc)

	nnet := NewNatsNet(cli)
	//fmt.Printf("recv.go is setting up NewSession...\n")
	to := time.Millisecond * 100
	windowby := int64(1 << 20)
	//windowby := -1
	sessA, err := NewSession(SessionConfig{Net: nnet, LocalInbox: mysubj, DestInbox: fromsubj,
		WindowMsgCount: 1000, WindowByteSz: windowby, Timeout: to, Clk: RealClk,
		NumFailedKeepAlivesBeforeClosing: -1,
	})
	if err != nil {
		return nil, err
	}

	//rep := ReportOnSubscription(pub.Scrip)
	//fmt.Printf("rep = %#v\n", rep)

	msgLimit := int64(1000)
	bytesLimit := int64(600000)
	sessA.Swp.Sender.FlowCt = &FlowCtrl{Flow: Flow{
		ReservedByteCap: 600000,
		ReservedMsgCap:  1000,
	}}
	SetSubscriptionLimits(cli.Scrip, msgLimit, bytesLimit)

	//n, err := sessA.Write(writeme)
	//fmt.Fprintf(os.Stderr, "n = %v, err=%v after A.Write(writeme), where len(writeme)=%v\n", n, err, len(writeme))
	//sessA.Stop()
	return sessA, nil
}
