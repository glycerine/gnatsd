/*
Package swp implements the same Sliding Window Protocol that
TCP uses for flow-control and reliable, ordered delivery.

The Nats event bus (https://nats.io/) is a
software model of a hardware multicast
switch. Nats provides multicast, but no guarantees of delivery
and no flow-control. This works fine as long as your
downstream read/subscribe capacity is larger than your
publishing rate.

If your nats publisher evers produces
faster than your subscriber can keep up, you may overrun
your buffers and drop messages. If your sender is local
and replaying a disk file of traffic over nats, you are
guanateed to exhaust even the largest of the internal
nats client buffers. In addition you may wish guaranteed
order of delivery (even with dropped messages), which
swp provides.

Hence swp was built to provide flow-control and reliable, ordered
delivery on top of the nats event bus. It reproduces the
TCP sliding window and flow-control mechanism in a
Session between two nats clients. It provides flow
control between exactly two nats endpoints; in many
cases this is sufficient to allow all subscribers to
keep up.  If you have a wide variation in consumer
performance, establish the rate-controlling
swp Session between your producer and your
slowest consumer.

There is also a Session.RegisterAsap() API that can be
used to obtain possibly-out-of-order and possibly-duplicated
but as-soon-as-possible delivery (similar to that which
nats give you natively), while retaining the
flow-control required to avoid client-buffer overrun.
This can be used in tandem with the main always-ordered-and-lossless
API if so desired.
*/
package swp

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glycerine/bchan"
	"github.com/glycerine/cryrand"
	"github.com/glycerine/idem"
	"github.com/tinylib/msgp/msgp"
)

// sliding window protocol
//
// Reference: pp118-120, Computer Networks: A Systems Approach
//  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.
//
// In addition to sliding window, we implement flow-control
// similar to how tcp does for throttling senders.
// See pp296-301 of Peterson and Davie.
//
// Most of the implementation is in sender.go and recv.go.

//go:generate msgp

//msgp:ignore TxqSlot RxqSlot Semaphore SenderState RecvState SWP Session NatsNet SimNet Network Session SWP SessionConfig TermConfig ReadRequest

// here so that msgp can know what it is
type TcpEvent int

// Packet is what is transmitted between Sender A and
// Receiver B, where A and B are the two endpoints in a
// given Session. (Endpoints are specified by the strings localInbox and
// destInbox in the NewSession constructor.)
//
// Packets also flow symmetrically from Sender B to Receiver A.
//
// Special packets have AckOnly, KeepAlive, or Closing
// flagged; otherwise normal packets are data
// segments that have neither of these flags
// set. Only normal data packets are tracked
// for timeout and retry purposes.
type Packet struct {
	From string
	Dest string

	// uniquely identify a file/session with a randomly
	// chosen then fixed nonce. See NewSessionNonce() to generate.
	FromSessNonce string
	DestSessNonce string

	// ArrivedAtDestTm is timestamped by
	// the receiver immediately when the
	// packet arrives at the Dest receiver.
	ArrivedAtDestTm time.Time

	// DataSendTm is stamped anew on each data send and retry
	// from the sender. Acks by the receiver are
	// stamped with the AckReplyTm field, and DataSendTm
	// is copied to the ack to make one-way estimates (if
	// your clocks are synced) viable. Currently we do
	// as TCP does and compute RTT based on the roundtrip
	// from the originating endpoint, which avoids issues
	// around clock skew.
	//
	// For the rationale for updating the timestamp on each
	// retry, see the discussion of the Karn/Partridge
	// algorithm, p302 of Peterson and Davie 1996.
	//
	// Summary: Accurate RTT requires accurate association of
	// ack with which retry it is responding to, and thus
	// time of the retry is the relevant one for RTT computation.
	// If internally you need to know when a data packet was
	// first/originally transmitted, see the TxqSlot.OrigSendTime
	// value.
	DataSendTm time.Time

	SeqNum int64

	// SeqRetry: 0, 1, 2: allow
	// accurate RTT estimates. Only on
	// data frames; acks/control will be negative.
	SeqRetry int64

	AckNum int64

	// AckRetry: which SeqRetry attempt for
	// the AckNum this AckNum is associated with.
	AckRetry   int64
	AckReplyTm time.Time

	// things like Fin, FinAck, DataAck, KeepAlive are
	// all TcpEvents.
	TcpEvent TcpEvent

	// like the byte count AdvertisedWindow in TCP, but
	// since nats has both byte and message count
	// limits, we want convey these instead.
	AvailReaderBytesCap int64
	AvailReaderMsgCap   int64

	// Estimate of the round-trip-time (RTT) from
	// the senders point of view. In nanoseconds.
	// Allows mostly passive recievers to have
	// an accurate view of the end-to-end RTT.
	FromRttEstNsec int64

	// Estimate of the standard deviation of
	// the round-trip-time from the senders
	// point of view. In nanoseconds.
	FromRttSdNsec int64

	// number of RTT observations in the From RTT
	// estimates above, also avoids double counting.
	FromRttN int64

	// CumulBytesTransmitted should give the total accumulated
	// count of bytes ever transmitted on this session
	// from `From` to `Dest`.
	// On data payloads, CumulBytesTransmitted allows
	// the receiver to figure out how
	// big any gaps are, so as to give accurate flow control
	// byte count info. The CumulBytesTransmitted count
	// should include this packet's len(Data), assuming
	// this is a data packet.
	CumulBytesTransmitted int64

	Data []byte

	// DataOffset tells us
	// where to start reading from in Data. It
	// allows us to have consumed only part of
	// a packet on one Read().
	DataOffset int

	// checksum of Data
	Blake2bChecksum []byte

	// those waiting for when this particular
	// Packet is acked by the
	// recipient can allocate a bchan.New(1) here and wait for a
	// channel receive on <-CliAcked.Ch
	CliAcked *bchan.Bchan `msg:"-"` // omit from serialization

	Accounting *ByteAccount `msg:"-"` // omit from serialization
}

// SWP holds the Sliding Window Protocol state
type SWP struct {
	Sender *SenderState
	Recver *RecvState
}

// NewSWP makes a new sliding window protocol manager, holding
// both sender and receiver components.
func NewSWP(net Network, windowMsgCount int64, windowByteCount int64,
	timeout time.Duration, inbox string, destInbox string, clk Clock, keepAliveInterval time.Duration, nonce string) *SWP {

	snd := NewSenderState(net, windowMsgCount, timeout, inbox, destInbox, clk, keepAliveInterval, nonce)
	rcv := NewRecvState(net, windowMsgCount, windowByteCount, timeout, inbox, snd, clk, nonce)
	swp := &SWP{
		Sender: snd,
		Recver: rcv,
	}

	return swp
}

type TerminatedError struct {
	Msg string
}

func (t *TerminatedError) Error() string {
	return t.Msg
}

// Session tracks a given point-to-point sesssion and its
// sliding window state for one of the end-points.
type Session struct {
	Cfg         *SessionConfig
	Swp         *SWP
	Destination string
	MyInbox     string

	Net               Network
	ReadMessagesCh    chan InOrderSeq
	AcceptReadRequest chan *ReadRequest

	// if terminated with error,
	// that error will live here. Retreive
	// with GetErr()
	exitErr error

	// Halt.Done.Chan is closed if session is terminated.
	// This will happen if the remote session stops
	// responding and is thus declared dead, as well
	// as after an explicit close.
	Halt *idem.Halter

	packetsConsumed                  uint64
	packetsSent                      uint64
	NumFailedKeepAlivesBeforeClosing int

	mut                sync.Mutex
	RemoteSenderClosed chan bool

	LocalSessNonce  string
	RemoteSessNonce string
}

// SessionConfig configures a Session.
type SessionConfig struct {

	// the network the use, NatsNet or SimNet
	Net Network

	// where we listen
	LocalInbox string

	// the remote destination topic for our messages
	DestInbox string

	// capacity of our receive buffers in message count
	WindowMsgCount int64

	// capacity of our receive buffers in byte count
	WindowByteSz int64

	// how often we wakeup and check
	// if packets need to be retried.
	Timeout time.Duration

	KeepAliveInterval time.Duration

	// set to -1 to disable auto-close. If
	// not set (or left at 0), then we default
	// to 50 (so after 50 keep-alive intervals
	// with no remote contact, we close the session).
	NumFailedKeepAlivesBeforeClosing int

	// the clock (real or simulated) to use
	Clk Clock

	TermCfg TermConfig
}

type TermConfig struct {
	// how long a window we use for termination
	// checking. Ignored if 0.
	TermWindowDur time.Duration

	// how many unacked packets we can see inside
	// TermWindowDur before giving up and terminating
	// the session. Ignored if 0.
	TermUnackedLimit int
}

// NewSession makes a new Session, and calls
// Swp.Start to begin the sliding-window-protocol.
//
// If windowByteSz is negative or less than windowMsgSz,
// we estimate a byte size based on 10kb messages and
// the given windowMsgSz. It is much better to give the
// aggregate byte limit you want specifically. Negative
// windowByteSz is merely a convenience for writing tests.
//
// The timeout parameter controls how often we wake up
// to check for packets that need to be retried.
//
// Packet timers (retry deadlines) are established
// using an exponential moving average + some standard
// deviations of the observed round-trip time of
// Ack packets. Retries reset the DataSentTm so
// there is never confusion as to which retry is
// being acked. The Ack packet method is not subject
// to two-clock skew because the same clock is used
// for both begin and end measurements.
//
func NewSession(cfg SessionConfig) (*Session, error) {

	if cfg.WindowMsgCount < 1 {
		return nil, fmt.Errorf("windowMsgSz must be 1 or more")
	}

	if cfg.WindowByteSz < cfg.WindowMsgCount {
		// guestimate
		cfg.WindowByteSz = cfg.WindowMsgCount * 10 * 1024
	}

	// set default; user can set to -1 to deactivate auto-close.
	if cfg.NumFailedKeepAlivesBeforeClosing == 0 {
		cfg.NumFailedKeepAlivesBeforeClosing = 50
	}

	if cfg.KeepAliveInterval == 0 {
		cfg.KeepAliveInterval = time.Millisecond * 500
	}
	nonce := NewSessionNonce()

	sess := &Session{
		Cfg: &cfg,
		Swp: NewSWP(cfg.Net, cfg.WindowMsgCount, cfg.WindowByteSz,
			cfg.Timeout, cfg.LocalInbox, cfg.DestInbox, cfg.Clk,
			cfg.KeepAliveInterval, nonce),
		MyInbox:     cfg.LocalInbox,
		Destination: cfg.DestInbox,
		Net:         cfg.Net,
		Halt:        idem.NewHalter(),
		NumFailedKeepAlivesBeforeClosing: cfg.NumFailedKeepAlivesBeforeClosing,
		RemoteSenderClosed:               make(chan bool),
		LocalSessNonce:                   nonce,
	}
	sess.Swp.Sender.NumFailedKeepAlivesBeforeClosing = cfg.NumFailedKeepAlivesBeforeClosing
	sess.Swp.Start(sess)
	sess.ReadMessagesCh = sess.Swp.Recver.ReadMessagesCh
	sess.AcceptReadRequest = sess.Swp.Recver.AcceptReadRequest

	return sess, nil
}

// Push sends a message packet, blocking until there
// is a flow-control slot available.
// The Push will block due to flow control to avoid
// over-running the receiving clients buffers.
//
// Upon return we are not guaranteed that the
// the message has reached the broker or the client.
// If you want to Flush() to the broker,
// use the PushGetAck() method instead.
// This will hurt throughput, but may be
// needed if you are going to shutdown
// right after the send (to avoid dropping
// the packet before it reaches the broker).
//
// You can use s.CountPacketsSentForTransfer() to get
// the total count of packets Push()-ed so far.
func (s *Session) Push(pack *Packet) {
	select {
	case s.Swp.Sender.BlockingSend <- pack:
		//p("%v Push succeeded on payload '%s' into BlockingSend", s.MyInbox, string(pack.Data))
		s.IncrPacketsSentForTransfer(1)
	case <-s.Swp.Sender.Halt.ReqStop.Chan:
		// give up, Sender is shutting down.
	}
}

// SelfConsumeForTesting sets up a reader to read all produced
// messages automatically. You can use CountPacketsReadConsumed() to
// see the total number consumed thus far.
func (s *Session) SelfConsumeForTesting() {
	go func() {
		for {
			select {
			case <-s.Swp.Recver.Halt.ReqStop.Chan:
				return
			case read := <-s.ReadMessagesCh:
				s.IncrPacketsReadConsumed(int64(len(read.Seq)))
			}
		}
	}()
}

// IncrementClockOnReceiveForTesting supports testing by
// incrementing the Clock automatically when a packet is
// received. Expects a SimClock to be in use, and so
// to avoid mixing testing and prod this call will panic
// if that assumption in violated.
func (s *Session) IncrementClockOnReceiveForTesting() {
	s.Swp.Recver.Clk.(*SimClock).Advance(0)
	s.Swp.Recver.testModeOn()
	s.Swp.Recver.testing.incrementClockOnReceive = true
}

// InWindow returns true iff seqno is in [min, max].
func InWindow(seqno, min, max int64) bool {
	if seqno < min {
		return false
	}
	if seqno > max {
		return false
	}
	return true
}

// Stop shutsdown the session
func (s *Session) Stop() {
	///p("%v Session.Stop called.", s.MyInbox)
	s.Swp.Stop()
	s.SetErr(s.Swp.Sender.GetErr())
	s.Halt.RequestStop()
	s.Halt.Done.Close()
}

// Stop the sliding window protocol
func (s *SWP) Stop() {
	s.Recver.Stop()
	s.Sender.Stop()
}

// Start the sliding window protocol
func (s *SWP) Start(sess *Session) {
	//q("SWP Start() called")
	s.Recver.Start()
	s.Sender.Start(sess)
}

// CountPacketsReadConsumed reports on how many packets
// the application has read from the session.
func (s *Session) CountPacketsReadConsumed() int64 {
	return int64(atomic.LoadUint64(&s.packetsConsumed))
}

// IncrPacketsReadConsumed increment packetsConsumed and return the new total.
func (s *Session) IncrPacketsReadConsumed(n int64) int64 {
	return int64(atomic.AddUint64(&s.packetsConsumed, uint64(n)))
}

// CountPacketsSentForTransfer reports on how many packets.
// the application has written to the session.
func (s *Session) CountPacketsSentForTransfer() int64 {
	return int64(atomic.LoadUint64(&s.packetsSent))
}

// IncrPacketsSentForTransfer increment packetsConsumed and return the new total.
func (s *Session) IncrPacketsSentForTransfer(n int64) int64 {
	return int64(atomic.AddUint64(&s.packetsSent, uint64(n)))
}

// RegisterAsap registers a call back channel,
// rcvUnordered, which will get *Packet that are
// unordered and possibly
// have gaps in their sequence (where packets
// where dropped). However the channel will see
// the packets as soon as possible. The session
// will still be flow controlled however, so
// if the receiver throttles the sender, packets
// may be delayed. Clients should be prepared
// to deal with duplicated, dropped, and mis-ordered packets
// on the rcvUnordered channel.
// The limit argument sets how many messages are queued before
// we drop the oldest.
func (s *Session) RegisterAsap(rcvUnordered chan *Packet, limit int64) error {
	s.Swp.Recver.setAsapHelper <- NewAsapHelper(rcvUnordered, limit)
	return nil
}

// ackCallbackFunc is used for testing: used in setPacketRecvCallback
type ackCallbackFunc func(pack *Packet)

// for testing, get a callback when packets are received.
// test-mode must have already been activated by a call
// to s.Swp.recver.testModeOn(), or we will panic.
func (s *Session) setPacketRecvCallback(cb ackCallbackFunc) {
	s.Swp.Recver.testing.ackCb = cb
}

type ReadRequest struct {
	Done chan bool
	N    int
	Err  error
	P    []byte
}

func NewReadRequest(p []byte) *ReadRequest {
	return &ReadRequest{
		Done: make(chan bool),
		P:    p,
	}
}

// Read implements io.Reader
func (s *Session) Read(fillme []byte) (n int, err error) {
	rr := NewReadRequest(fillme)
	//p("Read gets fillme with len %v. len(rr.P)=%v", len(fillme), len(rr.P))
	//p("rr=%p, s=%p", rr, s)
	select {
	case s.AcceptReadRequest <- rr:
		//p("Read: rr request accepted")
		// good, proceed to get reply
	case <-s.Halt.Done.Chan:
		return 0, ErrSessDone
	}
	// now get reply.
	select {
	case <-rr.Done:
		//p("Read: got rr.Done closed, rr.N=%v, rr.Err=%v", rr.N, rr.Err)
		return rr.N, rr.Err
	case <-s.Halt.Done.Chan:
		return 0, ErrSessDone
	}
}

// ByteAccount should be accessed with
// atomics to avoid data races.
type ByteAccount struct {
	NumBytesAcked int64
}

// Write implements io.Writer, chopping p into packet
// sized pieces if need be, and sending then in order
// over the flow-controlled Session s.
func (s *Session) Write(payload []byte) (n int, err error) {

	err = s.ConnectIfNeeded(s.Destination)
	if err != nil {
		return 0, err
	}

	// use atomics to access the ByteAccount.
	// We attach this to our packets so that
	// we don't get any residual left over acks
	// that may have arrived late from another Write
	// that happened before.
	var ba ByteAccount

	lenp := int64(len(payload))
	if lenp == 0 {
		return 0, nil
	}

	// At 1MB, gnatsd freaks. Keep it under 512KB.
	// sz := int64Min(s.Cfg.WindowByteSz, 1<<19)
	// or for easier debugging, an even 128K
	sz := int64Min(s.Cfg.WindowByteSz, 131072)

	npack := lenp / sz
	if lenp-npack*sz > 0 {
		npack++
	}
	ca := bchan.New(1)
	for i := int64(0); i < npack; i++ {
		pack := &Packet{
			From:       s.MyInbox,
			Dest:       s.Destination,
			Data:       payload[i*sz : int64Min((i+1)*sz, lenp)],
			Accounting: &ba,
			TcpEvent:   EventData,
		}
		if i == npack-1 {
			pack.CliAcked = ca
		}
		s.Push(pack)
		///p("%s Write() sent packet %v of %v", s.MyInbox, i, npack)
	}
	select {
	case <-ca.Ch:
		///p("we got end-to-end ack from receiver that all packets were delivered")
		ca.BcastAck()
	case <-time.After(10 * time.Second):
		p("problem in %s Write: timeout after 10 seconds waiting", s.MyInbox)
	case <-s.Halt.Done.Chan:
	}

	nba := int(atomic.LoadInt64(&ba.NumBytesAcked))
	return nba, s.GetErr()
}

func int64Min(a, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}

func (s *Session) SetErr(err error) {
	s.mut.Lock()
	s.exitErr = err
	s.mut.Unlock()
}

func (s *Session) GetErr() (err error) {
	s.mut.Lock()
	err = s.exitErr
	s.mut.Unlock()
	return
}

// BigFile represents the transmission of a file;
// size is limited only by memory. Since the primary
// use is to capture the state that is being held
// in memory, this is a reasonable approach.
type BigFile struct {
	Filepath    string
	SizeInBytes int64
	Blake2b     []byte
	SendTime    time.Time
	Data        []byte
}

func (sess *Session) RecvFile() (*BigFile, error) {
	bf := &BigFile{}
	err := msgp.Decode(sess, bf)
	return bf, err
}

func (sess *Session) SendFile(path string, writeme []byte, tm time.Time) (*BigFile, error) {
	bf := &BigFile{
		Filepath:    path,
		SendTime:    tm,
		SizeInBytes: int64(len(writeme)),
		Data:        writeme,
		Blake2b:     Blake2bOfBytes(writeme),
	}

	err := msgp.Encode(sess, bf)
	return bf, err
}

// SynAckAck is used as the encoded Data
// for EventSyn, EventSynAck, and EventEstabAck: connection setup.
type SynAckAck struct {

	// EventSyn => also conveys SessionNonce
	TcpEvent TcpEvent

	// SessionNonce identifies the file in this session (replaces port numbers).
	// Should always be present.  See NewSessionNonce() to generate.
	SessionNonce string

	// NextExpected should be 0 if fresh start;
	// i.e. we know nothing from prior evesdropping on
	// any prior multicast of this SessionNonce. Otherwise,
	// NextExpected and NackList convey knowledge of what we have
	// and don't have to allow the sender to skip
	// repeating the packets.
	//
	// This is the next serial number that the receiver has not received.
	//
	// Only present on TcpEvent == EventSynAck.
	NextExpected int64

	// NackList can be an empty slice.
	// Nacklist is a list of missing packets (on the receiver side) that we are aware of.
	// (Nack=negative acknowledgement).
	// Only present on TcpEvent == EventSynAck.
	NackList []int64
}

func NewSessionNonce() string {
	return cryrand.RandomStringWithUp(30)
}

// if not established, do the connect 3-way handshake
// to exchange Session nonces.
func (s *Session) ConnectIfNeeded(dest string) error {
	if s.RemoteSessNonce == "" {
		remoteNonce, err := s.Swp.Connect(dest)
		if err != nil {
			return err
		}
		s.RemoteSessNonce = remoteNonce
		return nil
	}
	return nil
}

func (swp *SWP) Connect(dest string) (remoteNonce string, err error) {
	return swp.Recver.Connect(dest)
}
