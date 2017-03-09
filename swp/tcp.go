package swp

import (
	"fmt"
)

//go:generate msgp

type TcpState int

const (
	// init sequence
	Closed      TcpState = 0
	Listen      TcpState = 1 // server, passive open.
	SynReceived TcpState = 2
	SynSent     TcpState = 3 // client, active open.

	Established TcpState = 4

	// close sequence
	FinWait1  TcpState = 5
	FinWait2  TcpState = 6
	Closing   TcpState = 7
	CloseWait TcpState = 8
	LastAck   TcpState = 9
)

type TcpEvent int

// TcpEvent

const (
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

	// close initiator sends Fin, enters FinWait1
	// responder or simultaneous close, enter CloseWait or Closing
	EventFin    TcpEvent = 8
	EventFinAck TcpEvent = 9

	EventReset TcpEvent = 10

	// common acks of data during Established state
	// aka AckOnly
	EventDataAck TcpEvent = 11
	EventData    TcpEvent = 12 // a data packet. Most common. state is Established.

	// a keepalive
	EventKeepAlive TcpEvent = 14
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
func (s *TcpState) UpdateTcp(e TcpEvent) TcpAction {
	switch *s {
	// init sequence
	case Closed:
		switch e {
		case EventStartListen:
			*s = Listen
		case EventStartConnect:
			*s = SynSent
			return SendSyn
		case EventReset:
			// stay in closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}
	case Listen:
		switch e {
		case EventSyn:
			*s = SynReceived
			return SendSynAck
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case SynReceived:
		switch e {
		case EventEstabAck:
			*s = Established
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
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
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case Established:
		switch e {
		case EventData, EventDataAck:
			// stay in Established
		case EventKeepAlive:
			return SendDataAck
		case EventStartClose:
			*s = FinWait1
			return SendFin
		case EventFin:
			*s = CloseWait
			return DoAppClose

		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case FinWait1:
		switch e {
		case EventFinAck:
			*s = FinWait2
		case EventFin:
			// simultaneous close
			*s = Closing
			return SendFinAck
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case FinWait2:
		switch e {
		case EventFin:
			*s = Closed
			return SendFinAck
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case Closing:
		switch e {
		case EventFinAck:
			*s = Closed
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}

	case CloseWait:
		switch e {
		case EventData:
			// ignore
		case EventDataAck:
			// ignore
		case EventApplicationClosed:
			*s = LastAck
			return SendFin
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}
	case LastAck:
		switch e {
		case EventFinAck:
			*s = Closed
		case EventReset:
			*s = Closed
		default:
			panic(fmt.Sprintf("invalid event %s from state %s", e, *s))
		}
	}
	return NoAction
}
