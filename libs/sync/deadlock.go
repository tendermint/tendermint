// +build deadlock

package sync

import (
	deadlock "github.com/sasha-s/go-deadlock"
)

// A Mutex is a mutual exclusion lock.
type Mutex struct {
	deadlock.Mutex
}

// An RWMutex is a reader/writer mutual exclusion lock.
type RWMutex struct {
	deadlock.RWMutex
}

// Closer implements a primitive to close a channel that signals process
// termination while allowing a caller to call Close multiple times safely. It
// should be used in cases where guarantees cannot be made about when and how
// many times closure is executed.
type Closer struct {
	closeOnce deadlock.Once
	doneCh    chan struct{}
}

// NewCloser returns a reference to a new Closer.
func NewCloser() *Closer {
	return &Closer{doneCh: make(chan struct{})}
}

// Done returns the internal done channel allowing the caller either block or wait
// for the Closer to be terminated/closed.
func (c *Closer) Done() <-chan struct{} {
	return c.doneCh
}

// Close gracefully closes the Closer. A caller should only call Close once, but
// it is safe to call it successive times.
func (c *Closer) Close() {
	c.closeOnce.Do(func() {
		close(c.doneCh)
	})
}
