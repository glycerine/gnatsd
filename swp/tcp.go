package swp

//go:generate msgp

type TcpState int

const (
	// init sequence
	Fresh       TcpState = 0
	Closed      TcpState = 1
	Listen      TcpState = 2 // server, passive open.
	SynReceived TcpState = 3
	SynSent     TcpState = 4 // client, active open.

	// the numbering is significant, since we'll
	// test that state is >= Established before
	// enforcing SessNonce matching (in recv.go).
	Established TcpState = 5

	// close sequence
	// if we timeout in this state, we go to Closed.
	CloseInitiatorHasSentFin TcpState = 6 // FinWait1

	// if we timeout in this state, we go to Closed.
	CloseResponderGotFin TcpState = 7 // CloseWait
)

// moved to swp.go so that msgp serialization
// would know the type of TcpEvent.
//type TcpEvent int

// TcpEvent

const (
	EventNil          TcpEvent = 0
	EventStartListen  TcpEvent = 1 // server enters Listen state
	EventStartConnect TcpEvent = 2

	// client enters SynSent state
	// server enters SynReceived state, client enters SynReceived (simultaneous open)
	EventSyn TcpEvent = 3

	// server enters SynReceived
	// client enters Established, does EventSendEstabAck
	EventSynAck TcpEvent = 4

	// client enters Established
	// server enters Established
	EventEstabAck TcpEvent = 5

	EventStartClose        TcpEvent = 6
	EventApplicationClosed TcpEvent = 7

	// close initiator sends Fin, enters CloseInitiatorHasSentFin
	// responder or simultaneous close, enter CloseResponderGotFin

	// EventFin also serves as Reset, since our shutdown
	// sequence is much simpler than actual TCP.

	EventFin    TcpEvent = 8
	EventFinAck TcpEvent = 9

	// common acks of data during Established state
	// aka AckOnly
	EventDataAck TcpEvent = 10
	EventData    TcpEvent = 11 // a data packet. Most common. state is Established.

	// a keepalive
	EventKeepAlive TcpEvent = 12
)

type TcpAction int

const (
	NoAction     TcpAction = 0
	SendSyn      TcpAction = 1
	SendSynAck   TcpAction = 2
	SendEstabAck TcpAction = 3
	SendFin      TcpAction = 4
	SendFinAck   TcpAction = 5
	DoAppClose   TcpAction = 6 // after App has closed, then SendFinAck
	SendDataAck  TcpAction = 7
)

// e is the received event
func (s *TcpState) UpdateTcp(e TcpEvent, fromState TcpState) TcpAction {

	switch *s {
	case Closed:
		// Closed is permanent. We won't move out of it.
		// Create a whole new swp session to start again.
		return SendFinAck

	// init sequence
	case Fresh:
		switch e {
		case EventStartListen:
			*s = Listen
		case EventStartConnect:
			*s = SynSent
			return SendSyn
		}
	case Listen:
		switch e {
		case EventSyn:
			*s = SynReceived
			return SendSynAck
		case EventFin:
			// shutdown before even got started.
			*s = CloseResponderGotFin
			return DoAppClose
		}

	case SynReceived:
		// passive open, server side. SynReceived is the state after Listen.
		// Also happens for simultaneous open.
		switch e {
		case EventEstabAck:
			*s = Established
		case EventKeepAlive:
			if fromState == Established {
				p("moving from SynReceived to Established based on a keep alive reporting the remote side is in Established; so we probably lost/dropped an EventEstabAck")
				*s = Established
			}
		case EventFin:
			// early close, but no worries.
			*s = CloseResponderGotFin
			return DoAppClose
		}

	case SynSent:
		switch e {
		case EventSynAck:
			*s = Established
			return SendEstabAck
		case EventSyn:
			// simultaneous open
			*s = SynReceived
			return SendEstabAck
		case EventFin:
			*s = CloseResponderGotFin
			return DoAppClose
		}

	case Established:
		switch e {
		case EventKeepAlive:
			return SendDataAck
		case EventStartClose:
			*s = CloseInitiatorHasSentFin
			return SendFin
		case EventFin:
			*s = CloseResponderGotFin
			return DoAppClose
		}

	case CloseInitiatorHasSentFin:
		// aka FinWait1
		switch e {
		case EventFinAck:
			*s = Closed
		case EventFin:
			// simultaneous close
			*s = Closed
		}

	case CloseResponderGotFin:
		// aka CloseWait
		switch e {
		case EventApplicationClosed:
			*s = Closed
		}
	}
	return NoAction
}
