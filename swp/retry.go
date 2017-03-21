package swp

import (
	"log"
	"time"
)

type tcpRetryLogic struct {
	curState             TcpState
	inUse                bool
	enteredCurTcpStateAt time.Time
	lastAttemptAt        time.Time
	firstStateAction     TcpAction
	attemptCount         int
	causalPacket         *Packet
	timeout              time.Duration
}

func isRetryState(state TcpState) bool {
	switch state {
	default:
		return false

	case SynSent, // resend Syn
		SynReceived, // resend SynAck
		FinWait1,    // resend Fin
		FinWait2,    // resend FinAck
		Closing:     // resend FinAck
		//CloseWait,   // resend FinAck
		//LastAck:     // resend Fin
		return true
	}
}

func (r *RecvState) retryCheck() {

	// retry our action?
	if !r.retry.inUse {
		r.retryTimerCh = nil
		return
	}

	if r.retry.curState != r.TcpState {
		r.retryTimerCh = nil
		r.retry.inUse = false
		r.retry.causalPacket = nil
		return
	}

	now := time.Now()
	elap := now.Sub(r.retry.lastAttemptAt)
	if elap > time.Second {
		r.retry.attemptCount++
		if r.retry.attemptCount > 4 {
			log.Printf("%s with LocalSessNonce %s, warning: retryCheck is failing after 4 tries, in state %s, trying to do action %s. Closing up shop.", r.Inbox, r.LocalSessNonce, r.TcpState, r.retry.firstStateAction)
			r.retryTimerCh = nil
			r.retry.inUse = false
			r.Halt.ReqStop.Close()
			return
		}

		log.Printf("%s retrying attempt %v, from "+
			"state %s, doing action %s. Elap since orig attempt %v",
			r.Inbox,
			r.retry.attemptCount,
			r.retry.curState,
			r.retry.firstStateAction, elap)

		// here is the retry:
		// (ignore errors from doTcpAction)
		r.doTcpAction(
			r.retry.firstStateAction,
			r.retry.causalPacket)

		r.retry.lastAttemptAt = now
		r.retry.timeout = r.retry.timeout * 2 // exponential backoff
		r.retryTimerCh = time.After(r.retry.timeout)
	}
}

func (r *RecvState) setupRetry(
	pre TcpState,
	post TcpState,
	pack *Packet,
	act TcpAction,
) {
	if post == pre {
		return
	}

	// we changed state, note the time now
	// so as to schedule retries.
	if isRetryState(r.TcpState) {
		now := time.Now()
		r.retry.inUse = true
		r.retry.curState = post
		r.retry.enteredCurTcpStateAt = now
		r.retry.lastAttemptAt = now
		r.retry.firstStateAction = act
		r.retry.causalPacket = pack
		r.retry.timeout = time.Second
		r.retryTimerCh = time.After(r.retry.timeout)
	} else {
		r.retryTimerCh = nil
		r.retry.inUse = false
		r.retry.causalPacket = nil
	}
}
