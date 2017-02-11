// from https://github.com/bradfitz/http2/pull/8/files
//
// motivation: https://groups.google.com/forum/#!topic/golang-dev/k0bSal8eDyE
//
// Copyright 2014 The Go Authors.
// See https://code.google.com/p/go/source/browse/CONTRIBUTORS
// Licensed under the same terms as Go itself:
// https://code.google.com/p/go/source/browse/LICENSE

// package pipe gives a local in-memory net.Conn
package lcon

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Pipe is buffered version of net.Pipe. Reads
// will block until data is available.
type Pipe struct {
	b       buffer
	rc      sync.Cond
	wc      sync.Cond
	rm      sync.Mutex
	wm      sync.Mutex
	Flushed chan bool

	readDeadline  time.Time
	writeDeadline time.Time
}

// NewPipe must be given a buf of
// pre-allocated size to use as the
// internal buffer between reads
// and writes.
func NewPipe(buf []byte) *Pipe {
	p := &Pipe{
		b:       buffer{buf: buf},
		Flushed: make(chan bool, 1),
	}
	p.rc = *sync.NewCond(&p.rm)
	return p
}

var ErrDeadline = fmt.Errorf("deadline exceeded")

// Read waits until data is available and copies bytes
// from the buffer into p.
func (r *Pipe) Read(p []byte) (n int, err error) {
	r.rc.L.Lock()
	defer r.rc.L.Unlock()
	if !r.readDeadline.IsZero() {
		now := time.Now()
		dur := r.readDeadline.Sub(now)
		if dur <= 0 {
			return 0, ErrDeadline
		}
		go func(dur time.Duration) {
			time.Sleep(dur)
			r.rc.L.Lock()
			r.b.late = true
			r.rc.L.Unlock()
			r.rc.Broadcast()
		}(dur)
	}
	for r.b.Len() == 0 && !r.b.closed && !r.b.late {
		r.rc.Wait()
	}
	return r.b.Read(p)
}

// Write copies bytes from p into the buffer and wakes a reader.
// It is an error to write more data than the buffer can hold.
func (w *Pipe) Write(p []byte) (n int, err error) {
	w.rc.L.Lock()
	defer w.rc.L.Unlock()
	if !w.writeDeadline.IsZero() {
		now := time.Now()
		dur := w.writeDeadline.Sub(now)
		if dur <= 0 {
			return 0, ErrDeadline
		}
		go func(dur time.Duration) {
			time.Sleep(dur)
			w.rc.L.Lock()
			w.b.late = true
			w.rc.L.Unlock()
			w.rc.Broadcast()
		}(dur)
	}
	defer w.rc.Signal()
	defer w.flush()
	return w.b.Write(p)
}

var ErrLconPipeClosed = fmt.Errorf("lcon pipe closed")

func (c *Pipe) Close() error {
	c.SetErrorAndClose(ErrLconPipeClosed)
	return nil
}

func (c *Pipe) SetErrorAndClose(err error) {
	c.rc.L.Lock()
	defer c.rc.L.Unlock()
	defer c.rc.Signal()
	c.b.Close(err)
}

// Pipe technically fullfills the net.Conn interface

func (c *Pipe) LocalAddr() net.Addr  { return addr{} }
func (c *Pipe) RemoteAddr() net.Addr { return addr{} }

func (c *Pipe) flush() {
	if len(c.Flushed) == 0 {
		c.Flushed <- true
	}
}

type addr struct{}

func (a addr) String() string  { return "memory.pipe:0" }
func (a addr) Network() string { return "in-process-internal" }

// SetDeadline implements the net.Conn method
func (c *Pipe) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	err2 := c.SetWriteDeadline(t)
	if err != nil {
		return err
	}
	return err2
}

// SetWriteDeadline implements the net.Conn method
func (c *Pipe) SetWriteDeadline(t time.Time) error {
	c.rc.L.Lock()
	c.writeDeadline = t
	c.rc.L.Unlock()
	return nil
}

// SetReadDeadline implements the net.Conn method
func (c *Pipe) SetReadDeadline(t time.Time) error {
	c.rc.L.Lock()
	c.readDeadline = t
	c.rc.L.Unlock()
	return nil
}
