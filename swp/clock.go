package swp

import (
	"sync"
	"time"
)

var RealClk RealClock

// Clock interface allows test code to control
// time advancement
type Clock interface {

	// Now provides the present (or simulated) time
	Now() time.Time
}

// SimClock simulates time passing. Call
// Advance to increment the time.
type SimClock struct {
	mu   sync.Mutex
	When time.Time
}

// Now provides the simulated current time.
func (c *SimClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.When
}

// Advance causes the simulated clock to advance by d.
func (c *SimClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.When = c.When.Add(d)
	return c.When
}

// Set sets the SimClock time to w, and w will
// be returned by Now() until another Set or Advance
// call is made.
func (c *SimClock) Set(w time.Time) {
	c.mu.Lock()
	c.When = w
	c.mu.Unlock()
}

// RealClock just passes the Now() call to time.Now().
type RealClock struct{}

// Now returns time.Now().
func (c RealClock) Now() time.Time {
	return time.Now()
}
