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
	Name   string
	Ch     <-chan struct{}
	input  chan throttleCommand
	output chan<- struct{}
	dur    time.Duration

	timer   *time.Timer
	isSet   bool
	stopped bool
}

type throttleCommand int32

const (
	Set throttleCommand = iota
	Unset
	TQuit
)

// NewThrottleTimer creates a new ThrottleTimer.
func NewThrottleTimer(name string, dur time.Duration) *ThrottleTimer {
	c := make(chan struct{})
	var t = &ThrottleTimer{
		Name:   name,
		Ch:     c,
		dur:    dur,
		input:  make(chan throttleCommand),
		output: c,
		timer:  time.NewTimer(dur),
	}
	t.timer.Stop()
	go t.run()
	return t
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
	case t.output <- struct{}{}:
		t.isSet = false
	default:
		// if we just want to drop, replace this with t.isSet = false
		t.timer.Reset(t.dur)
	}
}

// all modifications of the internal state of ThrottleTimer
// happen in this method. It is only called from the run goroutine
// so we avoid any race conditions
func (t *ThrottleTimer) processInput(cmd throttleCommand) (shutdown bool) {
	switch cmd {
	case Set:
		if !t.isSet {
			t.isSet = true
			t.timer.Reset(t.dur)
		}
	case TQuit:
		t.stopped = true
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

// Stop prevents the ThrottleTimer from firing. It always returns true. Stop does not
// close the channel, to prevent a read from the channel succeeding
// incorrectly.
//
// To prevent a timer created with NewThrottleTimer from firing after a call to
// Stop, check the return value and drain the channel.
//
// For example, assuming the program has not received from t.C already:
//
// 	if !t.Stop() {
// 		<-t.C
// 	}
//
// For ease of stopping services before starting them, we ignore Stop on nil
// ThrottleTimers.
func (t *ThrottleTimer) Stop() bool {
	if t == nil || t.stopped {
		return false
	}
	t.input <- TQuit
	return true
}
