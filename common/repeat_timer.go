package common

import (
	"sync"
	"time"
)

// Ticker is a basic ticker interface.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
	Reset()
}

// DefaultTicker wraps the stdlibs Ticker implementation.
type DefaultTicker struct {
	t   *time.Ticker
	dur time.Duration
}

// NewDefaultTicker returns a new DefaultTicker
func NewDefaultTicker(dur time.Duration) *DefaultTicker {
	return &DefaultTicker{
		time.NewTicker(dur),
		dur,
	}
}

// Implements Ticker
func (t *DefaultTicker) Chan() <-chan time.Time {
	return t.t.C
}

// Implements Ticker
func (t *DefaultTicker) Stop() {
	t.t.Stop()
	t.t = nil
}

// Implements Ticker
func (t *DefaultTicker) Reset() {
	t.t = time.NewTicker(t.dur)
}

// ManualTicker wraps a channel that can be manually sent on
type ManualTicker struct {
	ch chan time.Time
}

// NewManualTicker returns a new ManualTicker
func NewManualTicker(ch chan time.Time) *ManualTicker {
	return &ManualTicker{
		ch: ch,
	}
}

// Implements Ticker
func (t *ManualTicker) Chan() <-chan time.Time {
	return t.ch
}

// Implements Ticker
func (t *ManualTicker) Stop() {
	// noop
}

// Implements Ticker
func (t *ManualTicker) Reset() {
	// noop
}

//---------------------------------------------------------------------

/*
RepeatTimer repeatedly sends a struct{}{} to .Ch after each "dur" period.
It's good for keeping connections alive.
A RepeatTimer must be Stop()'d or it will keep a goroutine alive.
*/
type RepeatTimer struct {
	Ch chan time.Time

	mtx    sync.Mutex
	name   string
	ticker Ticker
	quit   chan struct{}
	wg     *sync.WaitGroup
}

// NewRepeatTimer returns a RepeatTimer with the DefaultTicker.
func NewRepeatTimer(name string, dur time.Duration) *RepeatTimer {
	ticker := NewDefaultTicker(dur)
	return NewRepeatTimerWithTicker(name, ticker)
}

// NewRepeatTimerWithTicker returns a RepeatTimer with the given ticker.
func NewRepeatTimerWithTicker(name string, ticker Ticker) *RepeatTimer {
	var t = &RepeatTimer{
		Ch:     make(chan time.Time),
		ticker: ticker,
		quit:   make(chan struct{}),
		wg:     new(sync.WaitGroup),
		name:   name,
	}
	t.wg.Add(1)
	go t.fireRoutine(t.ticker)
	return t
}

func (t *RepeatTimer) fireRoutine(ticker Ticker) {
	for {
		select {
		case t_ := <-ticker.Chan():
			t.Ch <- t_
		case <-t.quit:
			// needed so we know when we can reset t.quit
			t.wg.Done()
			return
		}
	}
}

// Wait the duration again before firing.
func (t *RepeatTimer) Reset() {
	t.Stop()

	t.mtx.Lock() // Lock
	defer t.mtx.Unlock()

	t.ticker.Reset()
	t.quit = make(chan struct{})
	t.wg.Add(1)
	go t.fireRoutine(t.ticker)
}

// For ease of .Stop()'ing services before .Start()'ing them,
// we ignore .Stop()'s on nil RepeatTimers.
func (t *RepeatTimer) Stop() bool {
	if t == nil {
		return false
	}
	t.mtx.Lock() // Lock
	defer t.mtx.Unlock()

	exists := t.ticker != nil
	if exists {
		t.ticker.Stop() // does not close the channel
		select {
		case <-t.Ch:
			// read off channel if there's anything there
		default:
		}
		close(t.quit)
		t.wg.Wait() // must wait for quit to close else we race Reset
	}
	return exists
}
