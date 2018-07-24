package common

import (
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
)

func TestDefaultTicker(t *testing.T) {
	ticker := defaultTickerMaker(time.Millisecond * 10)
	<-ticker.Chan()
	ticker.Stop()
}

func TestRepeatTimer(t *testing.T) {

	ch := make(chan time.Time, 100)
	mtx := new(sync.Mutex)

	// tick() fires from start to end
	// (exclusive) in milliseconds with incr.
	// It locks on mtx, so subsequent calls
	// run in series.
	tick := func(startMs, endMs, incrMs time.Duration) {
		mtx.Lock()
		go func() {
			for tMs := startMs; tMs < endMs; tMs += incrMs {
				lt := time.Time{}
				lt = lt.Add(tMs * time.Millisecond)
				ch <- lt
			}
			mtx.Unlock()
		}()
	}

	// tock consumes Ticker.Chan() events and checks them against the ms in "timesMs".
	tock := func(t *testing.T, rt *RepeatTimer, timesMs []int64) {

		// Check against timesMs.
		for _, timeMs := range timesMs {
			tyme := <-rt.Chan()
			sinceMs := tyme.Sub(time.Time{}) / time.Millisecond
			assert.Equal(t, timeMs, int64(sinceMs))
		}

		// TODO detect number of running
		// goroutines to ensure that
		// no other times will fire.
		// See https://github.com/tendermint/tendermint/libs/issues/120.
		time.Sleep(time.Millisecond * 100)
		done := true
		select {
		case <-rt.Chan():
			done = false
		default:
		}
		assert.True(t, done)
	}

	tm := NewLogicalTickerMaker(ch)
	rt := NewRepeatTimerWithTickerMaker("bar", time.Second, tm)

	/* NOTE: Useful for debugging deadlocks...
	go func() {
		time.Sleep(time.Second * 3)
		trace := make([]byte, 102400)
		count := runtime.Stack(trace, true)
		fmt.Printf("Stack of %d bytes: %s\n", count, trace)
	}()
	*/

	tick(0, 1000, 10)
	tock(t, rt, []int64{})
	tick(1000, 2000, 10)
	tock(t, rt, []int64{1000})
	tick(2005, 5000, 10)
	tock(t, rt, []int64{2005, 3005, 4005})
	tick(5001, 5999, 1)
	// Read 5005 instead of 5001 because
	// it's 1 second greater than 4005.
	tock(t, rt, []int64{5005})
	tick(6000, 7005, 1)
	tock(t, rt, []int64{6005})
	tick(7033, 8032, 1)
	tock(t, rt, []int64{7033})

	// After a reset, nothing happens
	// until two ticks are received.
	rt.Reset()
	tock(t, rt, []int64{})
	tick(8040, 8041, 1)
	tock(t, rt, []int64{})
	tick(9555, 9556, 1)
	tock(t, rt, []int64{9555})

	// After a stop, nothing more is sent.
	rt.Stop()
	tock(t, rt, []int64{})

	// Another stop panics.
	assert.Panics(t, func() { rt.Stop() })
}

func TestRepeatTimerReset(t *testing.T) {
	// check that we are not leaking any go-routines
	defer leaktest.Check(t)()

	timer := NewRepeatTimer("test", 20*time.Millisecond)
	defer timer.Stop()

	// test we don't receive tick before duration ms.
	select {
	case <-timer.Chan():
		t.Fatal("did not expect to receive tick")
	default:
	}

	timer.Reset()

	// test we receive tick after Reset is called
	select {
	case <-timer.Chan():
		// all good
	case <-time.After(40 * time.Millisecond):
		t.Fatal("expected to receive tick after reset")
	}

	// just random calls
	for i := 0; i < 100; i++ {
		time.Sleep(time.Duration(RandIntn(40)) * time.Millisecond)
		timer.Reset()
	}
}
