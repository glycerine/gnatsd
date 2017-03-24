package swp

import (
	"sync"

	"github.com/glycerine/idem"
	"github.com/glycerine/nats"
)

// NatsNet connects to nats using the Network interface.
type NatsNet struct {
	Cli  *NatsClient
	mut  sync.Mutex
	Halt *idem.Halter
}

// NewNatsNet makes a new NataNet based on an actual nats client.
func NewNatsNet(cli *NatsClient) *NatsNet {
	net := &NatsNet{
		Cli:  cli,
		Halt: idem.NewHalter(),
	}
	return net
}

// BufferCaps returns the byte and message limits
// currently in effect, so that flow control
// can be used to avoid sender overrunning them.
func (n *NatsNet) BufferCaps() (bytecap int64, msgcap int64) {
	n.mut.Lock()
	defer n.mut.Unlock()
	return GetSubscripCap(n.Cli.Scrip)
}

// Listen starts receiving packets addressed to inbox on the returned channel.
func (n *NatsNet) Listen(inbox string) (chan *Packet, error) {
	mr := make(chan *Packet)

	//p("%s NatsNet.Listen(inbox='%s') called... (prior n.Cli.Scrip='%#v') ... attempting subscription on inbox", n.Cli.Cfg.NatsNodeName, inbox, n.Cli.Scrip)

	// do actual subscription
	err := n.Cli.MakeSub(inbox, func(msg *nats.Msg) {
		var pack Packet
		_, err := pack.UnmarshalMsg(msg.Data)
		panicOn(err)
		select {
		case mr <- &pack:
		case <-n.Halt.ReqStop.Chan:
			//		case <-time.After(10 * time.Second):
			//			p("NatsNet dropping pack.SeqNum=%v after 10 seconds of failing to deliver to receiver", pack.SeqNum)
		}
	})
	//p("end of Listen(): subscription %v by %v on subject %v succeeded", n.Cli.Scrip.Subject, n.Cli.Cfg.NatsNodeName, inbox)
	return mr, err
}

// Send blocks until Send has started (but not until acked).
func (n *NatsNet) Send(pack *Packet, why string) error {
	//p("%s in NatsNet.Send(pack.SeqNum=%v / .AckNum=%v) why: '%s'", pack.From, pack.SeqNum, pack.AckNum, why)
	bts, err := pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	err = n.Cli.Nc.Publish(pack.Dest, bts)
	//p("%s in NatsNet.Send() about to Nc.Publish... err='%v'", pack.From, err)
	return err
}

func (n *NatsNet) Stop() {
	//p("NatsNet.Stop called!")
	n.Halt.RequestStop()
	n.Halt.Done.Close()
}

func (n *NatsNet) Flush() {
	n.Cli.Nc.Flush()
}
