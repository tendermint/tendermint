package common

import (
	"time"
)

/*
ThrottleTimer fires an event at most "dur" after each .Set() call.
If a short burst of .Set() calls happens, ThrottleTimer fires once.
If a long continuous burst of .Set() calls happens, ThrottleTimer fires
at most once every "dur".
*/
type ThrottleTimer struct {
	Name  string
	Ch    chan struct{}
	input chan command
	dur   time.Duration

	timer *time.Timer
	isSet bool
}

type command int32

const (
	Set command = iota
	Unset
	Quit
)

func NewThrottleTimer(name string, dur time.Duration) *ThrottleTimer {
	var t = &ThrottleTimer{
		Name:  name,
		Ch:    make(chan struct{}),
		dur:   dur,
		input: make(chan command),
		timer: time.NewTimer(dur),
	}
	t.timer.Stop()
	go t.run()
	return t
}

// C is the proper way to listen to the timer output.
// t.Ch will be made private in the (near?) future
func (t *ThrottleTimer) C() <-chan struct{} {
	return t.Ch
}

func (t *ThrottleTimer) run() {
	for {
		select {
		case cmd := <-t.input:
			// stop goroutine if the input says so
			// don't close channels, as closed channels mess up select reads
			if t.processInput(cmd) {
				return
			}
		case <-t.timer.C:
			t.trySend()
		}
	}
}

// trySend performs non-blocking send on t.Ch
func (t *ThrottleTimer) trySend() {
	select {
	case t.Ch <- struct{}{}:
		t.isSet = false
	default:
		// if we just want to drop, replace this with t.isSet = false
		t.timer.Reset(t.dur)
	}
}

// all modifications of the internal state of ThrottleTimer
// happen in this method. It is only called from the run goroutine
// so we avoid any race conditions
func (t *ThrottleTimer) processInput(cmd command) (shutdown bool) {
	switch cmd {
	case Set:
		if !t.isSet {
			t.isSet = true
			t.timer.Reset(t.dur)
		}
	case Quit:
		shutdown = true
		fallthrough
	case Unset:
		if t.isSet {
			t.isSet = false
			t.timer.Stop()
		}
	default:
		panic("unknown command!")
	}
	return shutdown
}

func (t *ThrottleTimer) Set() {
	t.input <- Set
}

func (t *ThrottleTimer) Unset() {
	t.input <- Unset
}

// For ease of .Stop()'ing services before .Start()'ing them,
// we ignore .Stop()'s on nil ThrottleTimers
func (t *ThrottleTimer) Stop() bool {
	if t == nil {
		return false
	}
	t.input <- Quit
	return true
}
