package swp

import (
	"fmt"
	"io"
	"time"
)

// EventRingBuf:
//
//  a fixed-size circular ring buffer of interface{}
//
type EventRingBuf struct {
	A        []time.Time
	N        int // MaxView, the total size of A, whether or not in use.
	Window   time.Duration
	Beg      int // start of in-use data in A
	Readable int // number of pointers available in A (in use)
}

func (r *EventRingBuf) String() string {
	c1, c2 := r.TwoContig(false)
	s := ""
	for i := range c1 {
		s += fmt.Sprintf("%v, ", c1[i])
	}
	for i := range c2 {
		s += fmt.Sprintf("%v, ", c2[i])
	}
	return s
}

// AddEventCheckOverflow adds event e and removes any events that are older
// than event e in the ring by more than Window.
// Returns true if the ring is full after the addition of e.
// Hence a true response indicates we have seen N events
// within Window of time.
func (b *EventRingBuf) AddEventCheckOverflow(e time.Time) bool {
	writeStart := (b.Beg + b.Readable) % b.N

	youngest := writeStart
	space := b.N - b.Readable
	//p("youngest = %v", youngest)
	b.A[youngest] = e
	if space == 0 {
		b.Beg = (b.Beg + 1) % b.N
	} else {
		b.Readable++
	}
	oldest := b.Beg
	left := b.N - b.Readable
	if left > 0 {
		// haven't seen N events yet,
		// so can't overflow
		return false
	}

	// at capacity, check time distance from newest to oldest
	if b.A[youngest].Sub(b.A[oldest]) <= b.Window {
		//p("b.A[youngest]=%v  b.A[oldest]=%v   b.Window = %v,   sub=%v", b.A[youngest], b.A[oldest], b.Window, b.A[youngest].Sub(b.A[oldest]))
		return true
	}
	return false
}

// constructor. NewEventRingBuf will allocate internally
// a slice of size maxViewInBytes.
func NewEventRingBuf(maxEventsStored int, maxTimeWindow time.Duration) *EventRingBuf {
	n := maxEventsStored
	r := &EventRingBuf{
		N:        n,
		Beg:      0,
		Readable: 0,
		Window:   maxTimeWindow,
	}
	r.A = make([]time.Time, n, n+1)

	return r
}

// TwoContig returns all readable pointers, but in two separate slices,
// to avoid copying. The two slices are from the same buffer, but
// are not contiguous. Either or both may be empty slices.
func (b *EventRingBuf) TwoContig(makeCopy bool) (first []time.Time, second []time.Time) {

	extent := b.Beg + b.Readable
	if extent <= b.N {
		// we fit contiguously in this buffer without wrapping to the other.
		// Let second stay an empty slice.
		return b.A[b.Beg:(b.Beg + b.Readable)], second
	}
	one := b.A[b.Beg:b.N]
	two := b.A[0:(extent % b.N)]
	return one, two
}

// ReadPtrs():
//
// from bytes.Buffer.Read(): Read reads the next len(p) time.Time
// pointers from the buffer or until the buffer is drained. The return
// value n is the number of bytes read. If the buffer has no data
// to return, err is io.EOF (unless len(p) is zero); otherwise it is nil.
func (b *EventRingBuf) ReadPtrs(p []time.Time) (n int, err error) {
	return b.readAndMaybeAdvance(p, true)
}

// ReadWithoutAdvance(): if you want to Read the data and leave
// it in the buffer, so as to peek ahead for example.
func (b *EventRingBuf) ReadWithoutAdvance(p []time.Time) (n int, err error) {
	return b.readAndMaybeAdvance(p, false)
}

func (b *EventRingBuf) readAndMaybeAdvance(p []time.Time, doAdvance bool) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.Readable == 0 {
		return 0, io.EOF
	}
	extent := b.Beg + b.Readable
	if extent <= b.N {
		n += copy(p, b.A[b.Beg:extent])
	} else {
		n += copy(p, b.A[b.Beg:b.N])
		if n < len(p) {
			n += copy(p[n:], b.A[0:(extent%b.N)])
		}
	}
	if doAdvance {
		b.Advance(n)
	}
	return
}

//
// WritePtrs writes len(p) time.Time values from p to
// the underlying ring, b.A.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// Write must return a non-nil error if it returns n < len(p).
//
func (b *EventRingBuf) WritePtrs(p []time.Time) (n int, err error) {
	for {
		if len(p) == 0 {
			// nothing (left) to copy in; notice we shorten our
			// local copy p (below) as we read from it.
			return
		}

		writeCapacity := b.N - b.Readable
		if writeCapacity <= 0 {
			// we are all full up already.
			return n, io.ErrShortWrite
		}
		if len(p) > writeCapacity {
			err = io.ErrShortWrite
			// leave err set and
			// keep going, write what we can.
		}

		writeStart := (b.Beg + b.Readable) % b.N

		upperLim := intMin(writeStart+writeCapacity, b.N)

		k := copy(b.A[writeStart:upperLim], p)

		n += k
		b.Readable += k
		p = p[k:]

		// we can fill from b.A[0:something] from
		// p's remainder, so loop
	}
}

// Reset quickly forgets any data stored in the ring buffer. The
// data is still there, but the ring buffer will ignore it and
// overwrite those buffers as new data comes in.
func (b *EventRingBuf) Reset() {
	b.Beg = 0
	b.Readable = 0
}

// Advance(): non-standard, but better than Next(),
// because we don't have to unwrap our buffer and pay the cpu time
// for the copy that unwrapping may need.
// Useful in conjuction/after ReadWithoutAdvance() above.
func (b *EventRingBuf) Advance(n int) {
	if n <= 0 {
		return
	}
	if n > b.Readable {
		n = b.Readable
	}
	b.Readable -= n
	b.Beg = (b.Beg + n) % b.N
}

// Adopt(): non-standard.
//
// For efficiency's sake, (possibly) take ownership of
// already allocated slice offered in me.
//
// If me is large we will adopt it, and we will potentially then
// write to the me buffer.
// If we already have a bigger buffer, copy me into the existing
// buffer instead.
func (b *EventRingBuf) Adopt(me []time.Time) {
	n := len(me)
	if n > b.N {
		b.A = me
		b.N = n
		b.Beg = 0
		b.Readable = n
	} else {
		// we already have a larger buffer, reuse it.
		copy(b.A, me)
		b.Beg = 0
		b.Readable = n
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func intMin(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}
