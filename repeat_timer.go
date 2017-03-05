package common

import (
	"sync"
	"time"
)

/*
RepeatTimer repeatedly sends a struct{}{} to .Ch after each "dur" period.
It's good for keeping connections alive.
A RepeatTimer must be Stop()'d or it will keep a goroutine alive.
*/
type RepeatTimer struct {
	Ch chan time.Time

	mtx    sync.Mutex
	name   string
	ticker *time.Ticker
	quit   chan struct{}
	wg     *sync.WaitGroup
	dur    time.Duration
}

func NewRepeatTimer(name string, dur time.Duration) *RepeatTimer {
	var t = &RepeatTimer{
		Ch:     make(chan time.Time),
		ticker: time.NewTicker(dur),
		quit:   make(chan struct{}),
		wg:     new(sync.WaitGroup),
		name:   name,
		dur:    dur,
	}
	t.wg.Add(1)
	go t.fireRoutine(t.ticker)
	return t
}

func (t *RepeatTimer) fireRoutine(ticker *time.Ticker) {
	for {
		select {
		case t_ := <-ticker.C:
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

	t.ticker = time.NewTicker(t.dur)
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
		t.ticker = nil
	}
	return exists
}
