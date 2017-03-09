package main

import (
	swp "github.com/glycerine/hnatsd/swp"
	"time"
)

func main() {

	host := "127.0.0.1"
	port := getAvailPort()
	gnats := swp.StartGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()

	// ===============================
	// setup nats clients for a publisher and a subscriber
	// ===============================

	subC := NewNatsClientConfig(host, port, "B", "B", true, false)
	sub := NewNatsClient(subC)
	err = sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A", "A", true, false)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
	defer pub.Close()

	// ===============================
	// make a session for each
	// ===============================

	anet := NewNatsNet(pub)
	bnet := NewNatsNet(sub)

	q("sub = %#v", sub)
	q("pub = %#v", pub)

	//lossProb := float64(0)
	lat := 1 * time.Millisecond

	rtt := 100 * lat

	A, err := NewSession(SessionConfig{Net: anet, LocalInbox: "A", DestInbox: "B",
		WindowMsgSz: 1, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)
	p("receiver only wants 1 at a time")
	// for some reason 1 at a time thrases the semaphores
	// somewhere in the Go runtime.
	B, err := NewSession(SessionConfig{Net: bnet, LocalInbox: "B", DestInbox: "A",
		WindowMsgSz: 1, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
	panicOn(err)

}

// getAvailPort asks the OS for an unused port.
// There's a race here, where the port could be grabbed by someone else
// before the caller gets to Listen on it, but in practice such races
// are rare. Uses net.Listen("tcp", ":0") to determine a free port, then
// releases it back to the OS with Listener.Close().
func getAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}
