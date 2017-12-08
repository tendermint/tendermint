package common

import (
	"time"
)

/*
RepeatTimer repeatedly sends a struct{}{} to .Ch after each "dur" period.
It's good for keeping connections alive.
A RepeatTimer must be Stop()'d or it will keep a goroutine alive.
*/
type RepeatTimer struct {
	Name   string
	Ch     <-chan time.Time
	output chan<- time.Time
	input  chan repeatCommand

	dur     time.Duration
	ticker  *time.Ticker
	stopped bool
}

type repeatCommand int8

const (
	Reset repeatCommand = iota
	RQuit
)

func NewRepeatTimer(name string, dur time.Duration) *RepeatTimer {
	c := make(chan time.Time)
	var t = &RepeatTimer{
		Name:   name,
		Ch:     c,
		output: c,
		input:  make(chan repeatCommand),

		dur:    dur,
		ticker: time.NewTicker(dur),
	}
	go t.run()
	return t
}

// Wait the duration again before firing.
func (t *RepeatTimer) Reset() {
	t.input <- Reset
}

// For ease of .Stop()'ing services before .Start()'ing them,
// we ignore .Stop()'s on nil RepeatTimers.
func (t *RepeatTimer) Stop() bool {
	// use t.stopped to gracefully handle many Stop() without blocking
	if t == nil || t.stopped {
		return false
	}
	t.input <- RQuit
	t.stopped = true
	return true
}

func (t *RepeatTimer) run() {
	done := false
	for !done {
		select {
		case cmd := <-t.input:
			// stop goroutine if the input says so
			// don't close channels, as closed channels mess up select reads
			done = t.processInput(cmd)
		case tick := <-t.ticker.C:
			t.send(tick)
		}
	}
}

// send performs blocking send on t.Ch
func (t *RepeatTimer) send(tick time.Time) {
	// XXX: possibly it is better to not block:
	// https://golang.org/src/time/sleep.go#L132
	// select {
	// case t.output <- tick:
	// default:
	// }
	t.output <- tick
}

// all modifications of the internal state of ThrottleTimer
// happen in this method. It is only called from the run goroutine
// so we avoid any race conditions
func (t *RepeatTimer) processInput(cmd repeatCommand) (shutdown bool) {
	switch cmd {
	case Reset:
		t.ticker.Stop()
		t.ticker = time.NewTicker(t.dur)
	case RQuit:
		t.ticker.Stop()
		shutdown = true
	default:
		panic("unknown command!")
	}
	return shutdown
}
