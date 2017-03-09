package swp

import (
	"fmt"
	"github.com/glycerine/nats"
)

// SubReport is the output of ReportOnSubscriptions.
type SubReport struct {
	Delivered          int64
	Dropped            int
	MaxMsgsQueued      int
	MaxBytesQueued     int
	PendingMsg         int
	PendingBytes       int
	LimitMsg           int
	LimitBytes         int
	SubscriptionActive bool
}

// ReportOnSubscription describes the status of the
// nats subscription s in terms of a SubReport with
// details the backing memory used and how close to
// the limits the client currently is.
func ReportOnSubscription(s *nats.Subscription) *SubReport {

	// Delivered returns the number of delivered messages for this subscription.
	ndeliv, err := s.Delivered()
	panicOn(err)

	// Dropped returns the number of known dropped messages for this
	// subscription. This will correspond to messages dropped by
	// violations of PendingLimits. If the server declares the
	// connection a SlowConsumer, this number may not be valid.
	ndrop, err := s.Dropped()
	panicOn(err)

	// IsValid returns a boolean indicating whether the subscription
	// is still active. This will return false if the subscription has
	// already been closed.
	activeSub := s.IsValid()

	// MaxPending returns the maximum number of queued messages and
	// queued bytes seen so far.
	maxMsgQueued, maxBytesQueued, err := s.MaxPending()
	panicOn(err)

	// Pending returns the number of queued messages and queued
	// bytes in the client for this subscription.
	pendMsg, pendBytes, err := s.Pending()
	panicOn(err)

	// PendingLimits returns the current limits for this subscription.
	msgLim, byteLim, err := s.PendingLimits()
	panicOn(err)

	sr := &SubReport{
		Delivered:          ndeliv,
		Dropped:            ndrop,
		MaxMsgsQueued:      maxMsgQueued,
		MaxBytesQueued:     maxBytesQueued,
		PendingMsg:         pendMsg,
		PendingBytes:       pendBytes,
		LimitMsg:           msgLim,
		LimitBytes:         byteLim,
		SubscriptionActive: activeSub,
	}

	return sr
}

// SetSubscriptionLimits changes the limits in the subscription sub.
func SetSubscriptionLimits(sub *nats.Subscription,
	msgLimit int64,
	bytesLimit int64) error {

	err := sub.SetPendingLimits(int(msgLimit), int(bytesLimit))
	if err != nil {
		return fmt.Errorf("Got an error on sub.SetPendingLimit(%v, %v)"+
			": %v", msgLimit, bytesLimit, err)
	}
	return nil
}

// GetSubscipCaps returns the byte and messge count
// capacity left for this subscription.
func GetSubscripCap(s *nats.Subscription) (bytecap int64, msgcap int64) {

	// Pending returns the number of queued messages and queued
	// bytes in the client for this subscription.
	pendMsg, pendBytes, err := s.Pending()
	panicOn(err)

	// PendingLimits returns the current limits for this subscription.
	msgLim, byteLim, err := s.PendingLimits()
	panicOn(err)

	advReaderBytesCap := int64(byteLim - pendBytes)
	advReaderMsgCap := int64(msgLim - pendMsg)

	return advReaderBytesCap, advReaderMsgCap
}
