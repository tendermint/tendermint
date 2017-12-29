package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTicker(t *testing.T) {
	ticker := defaultTickerMaker(time.Millisecond * 10)
	<-ticker.Chan()
	ticker.Stop()
}

func TestRepeat(t *testing.T) {

	ch := make(chan time.Time, 100)
	lt := time.Time{} // zero time is year 1

	// tick fires `cnt` times for each second.
	tick := func(cnt int) {
		for i := 0; i < cnt; i++ {
			lt = lt.Add(time.Second)
			ch <- lt
		}
	}

	// tock consumes Ticker.Chan() events `cnt` times.
	tock := func(t *testing.T, rt *RepeatTimer, cnt int) {
		for i := 0; i < cnt; i++ {
			timeout := time.After(time.Second * 10)
			select {
			case <-rt.Chan():
			case <-timeout:
				panic("expected RepeatTimer to fire")
			}
		}
		done := true
		select {
		case <-rt.Chan():
			done = false
		default:
		}
		assert.True(t, done)
	}

	tm := NewLogicalTickerMaker(ch)
	dur := time.Duration(10 * time.Millisecond) // less than a second
	rt := NewRepeatTimerWithTickerMaker("bar", dur, tm)

	// Start at 0.
	tock(t, rt, 0)
	tick(1) // init time

	tock(t, rt, 0)
	tick(1) // wait 1 periods
	tock(t, rt, 1)
	tick(2) // wait 2 periods
	tock(t, rt, 2)
	tick(3) // wait 3 periods
	tock(t, rt, 3)
	tick(4) // wait 4 periods
	tock(t, rt, 4)

	// Multiple resets leads to no firing.
	for i := 0; i < 20; i++ {
		time.Sleep(time.Millisecond)
		rt.Reset()
	}

	// After this, it works as new.
	tock(t, rt, 0)
	tick(1) // init time

	tock(t, rt, 0)
	tick(1) // wait 1 periods
	tock(t, rt, 1)
	tick(2) // wait 2 periods
	tock(t, rt, 2)
	tick(3) // wait 3 periods
	tock(t, rt, 3)
	tick(4) // wait 4 periods
	tock(t, rt, 4)

	// After a stop, nothing more is sent.
	rt.Stop()
	tock(t, rt, 0)

	// Another stop panics.
	assert.Panics(t, func() { rt.Stop() })
}
