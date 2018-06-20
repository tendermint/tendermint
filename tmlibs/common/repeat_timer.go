package common

import (
	"sync"
	"time"
)

// Used by RepeatTimer the first time,
// and every time it's Reset() after Stop().
type TickerMaker func(dur time.Duration) Ticker

// Ticker is a basic ticker interface.
type Ticker interface {

	// Never changes, never closes.
	Chan() <-chan time.Time

	// Stopping a stopped Ticker will panic.
	Stop()
}

//----------------------------------------
// defaultTicker

var _ Ticker = (*defaultTicker)(nil)

type defaultTicker time.Ticker

func defaultTickerMaker(dur time.Duration) Ticker {
	ticker := time.NewTicker(dur)
	return (*defaultTicker)(ticker)
}

// Implements Ticker
func (t *defaultTicker) Chan() <-chan time.Time {
	return t.C
}

// Implements Ticker
func (t *defaultTicker) Stop() {
	((*time.Ticker)(t)).Stop()
}

//----------------------------------------
// LogicalTickerMaker

// Construct a TickerMaker that always uses `source`.
// It's useful for simulating a deterministic clock.
func NewLogicalTickerMaker(source chan time.Time) TickerMaker {
	return func(dur time.Duration) Ticker {
		return newLogicalTicker(source, dur)
	}
}

type logicalTicker struct {
	source <-chan time.Time
	ch     chan time.Time
	quit   chan struct{}
}

func newLogicalTicker(source <-chan time.Time, interval time.Duration) Ticker {
	lt := &logicalTicker{
		source: source,
		ch:     make(chan time.Time),
		quit:   make(chan struct{}),
	}
	go lt.fireRoutine(interval)
	return lt
}

// We need a goroutine to read times from t.source
// and fire on t.Chan() when `interval` has passed.
func (t *logicalTicker) fireRoutine(interval time.Duration) {
	source := t.source

	// Init `lasttime`
	lasttime := time.Time{}
	select {
	case lasttime = <-source:
	case <-t.quit:
		return
	}
	// Init `lasttime` end

	for {
		select {
		case newtime := <-source:
			elapsed := newtime.Sub(lasttime)
			if interval <= elapsed {
				// Block for determinism until the ticker is stopped.
				select {
				case t.ch <- newtime:
				case <-t.quit:
					return
				}
				// Reset timeleft.
				// Don't try to "catch up" by sending more.
				// "Ticker adjusts the intervals or drops ticks to make up for
				// slow receivers" - https://golang.org/pkg/time/#Ticker
				lasttime = newtime
			}
		case <-t.quit:
			return // done
		}
	}
}

// Implements Ticker
func (t *logicalTicker) Chan() <-chan time.Time {
	return t.ch // immutable
}

// Implements Ticker
func (t *logicalTicker) Stop() {
	close(t.quit) // it *should* panic when stopped twice.
}

//---------------------------------------------------------------------

/*
	RepeatTimer repeatedly sends a struct{}{} to `.Chan()` after each `dur`
	period. (It's good for keeping connections alive.)
	A RepeatTimer must be stopped, or it will keep a goroutine alive.
*/
type RepeatTimer struct {
	name string
	ch   chan time.Time
	tm   TickerMaker

	mtx    sync.Mutex
	dur    time.Duration
	ticker Ticker
	quit   chan struct{}
}

// NewRepeatTimer returns a RepeatTimer with a defaultTicker.
func NewRepeatTimer(name string, dur time.Duration) *RepeatTimer {
	return NewRepeatTimerWithTickerMaker(name, dur, defaultTickerMaker)
}

// NewRepeatTimerWithTicker returns a RepeatTimer with the given ticker
// maker.
func NewRepeatTimerWithTickerMaker(name string, dur time.Duration, tm TickerMaker) *RepeatTimer {
	var t = &RepeatTimer{
		name:   name,
		ch:     make(chan time.Time),
		tm:     tm,
		dur:    dur,
		ticker: nil,
		quit:   nil,
	}
	t.reset()
	return t
}

// receive ticks on ch, send out on t.ch
func (t *RepeatTimer) fireRoutine(ch <-chan time.Time, quit <-chan struct{}) {
	for {
		select {
		case tick := <-ch:
			select {
			case t.ch <- tick:
			case <-quit:
				return
			}
		case <-quit: // NOTE: `t.quit` races.
			return
		}
	}
}

func (t *RepeatTimer) Chan() <-chan time.Time {
	return t.ch
}

func (t *RepeatTimer) Stop() {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	t.stop()
}

// Wait the duration again before firing.
func (t *RepeatTimer) Reset() {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	t.reset()
}

//----------------------------------------
// Misc.

// CONTRACT: (non-constructor) caller should hold t.mtx.
func (t *RepeatTimer) reset() {
	if t.ticker != nil {
		t.stop()
	}
	t.ticker = t.tm(t.dur)
	t.quit = make(chan struct{})
	go t.fireRoutine(t.ticker.Chan(), t.quit)
}

// CONTRACT: caller should hold t.mtx.
func (t *RepeatTimer) stop() {
	if t.ticker == nil {
		/*
			Similar to the case of closing channels twice:
			https://groups.google.com/forum/#!topic/golang-nuts/rhxMiNmRAPk
			Stopping a RepeatTimer twice implies that you do
			not know whether you are done or not.
			If you're calling stop on a stopped RepeatTimer,
			you probably have race conditions.
		*/
		panic("Tried to stop a stopped RepeatTimer")
	}
	t.ticker.Stop()
	t.ticker = nil
	/*
		From https://golang.org/pkg/time/#Ticker:
		"Stop the ticker to release associated resources"
		"After Stop, no more ticks will be sent"
		So we shouldn't have to do the below.

		select {
		case <-t.ch:
			// read off channel if there's anything there
		default:
		}
	*/
	close(t.quit)
}
