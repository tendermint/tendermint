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
		Ch:    make(chan struct{}, 1),
		dur:   dur,
		input: make(chan command),
		timer: time.NewTimer(dur),
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
			if t.processInput(cmd) {
				// TODO: do we want to close the channels???
				// close(t.Ch)
				// close(t.input)
				return
			}
		case <-t.timer.C:
			t.isSet = false
			t.Ch <- struct{}{}
		}
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
	// return true
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
