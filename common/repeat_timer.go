package common

import (
	"fmt"
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

	dur   time.Duration
	timer *time.Timer
}

type repeatCommand int32

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

		timer: time.NewTimer(dur),
		dur:   dur,
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
	if t == nil {
		return false
	}
	t.input <- RQuit
	return true
}

func (t *RepeatTimer) run() {
	for {
		fmt.Println("for")
		select {
		case cmd := <-t.input:
			// stop goroutine if the input says so
			// don't close channels, as closed channels mess up select reads
			if t.processInput(cmd) {
				t.timer.Stop()
				return
			}
		case <-t.timer.C:
			fmt.Println("tick")
			// send if not blocked, then start the next tick
			// for blocking send, just
			// t.output <- time.Now()
			t.trySend()
			t.timer.Reset(t.dur)
		}
	}
}

// trySend performs non-blocking send on t.Ch
func (t *RepeatTimer) trySend() {
	// TODO: this was blocking in previous version (t.Ch <- t_)
	// should I use that behavior unstead of unblocking as per throttle?
	select {
	case t.output <- time.Now():
	default:
	}
}

// all modifications of the internal state of ThrottleTimer
// happen in this method. It is only called from the run goroutine
// so we avoid any race conditions
func (t *RepeatTimer) processInput(cmd repeatCommand) (shutdown bool) {
	fmt.Printf("process: %d\n", cmd)
	switch cmd {
	case Reset:
		t.timer.Reset(t.dur)
	case RQuit:
		t.timer.Stop()
		shutdown = true
	default:
		panic("unknown command!")
	}
	return shutdown
}
