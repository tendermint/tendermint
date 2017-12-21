package common

import (
	"testing"
	"time"

	// make govet noshadow happy...
	asrt "github.com/stretchr/testify/assert"
)

// NOTE: this only tests with the ManualTicker.
// How do you test a real-clock ticker properly?
func TestRepeat(test *testing.T) {
	assert := asrt.New(test)

	ch := make(chan time.Time, 100)
	// tick fires cnt times on ch
	tick := func(cnt int) {
		for i := 0; i < cnt; i++ {
			ch <- time.Now()
		}
	}
	tock := func(test *testing.T, t *RepeatTimer, cnt int) {
		for i := 0; i < cnt; i++ {
			after := time.After(time.Second * 2)
			select {
			case <-t.Ch:
			case <-after:
				test.Fatal("expected ticker to fire")
			}
		}
		done := true
		select {
		case <-t.Ch:
			done = false
		default:
		}
		assert.True(done)
	}

	ticker := NewManualTicker(ch)
	t := NewRepeatTimerWithTicker("bar", ticker)

	// start at 0
	tock(test, t, 0)

	// wait for 4 periods
	tick(4)
	tock(test, t, 4)

	// keep reseting leads to no firing
	for i := 0; i < 20; i++ {
		time.Sleep(time.Millisecond)
		t.Reset()
	}
	tock(test, t, 0)

	// after this, it still works normal
	tick(2)
	tock(test, t, 2)

	// after a stop, nothing more is sent
	stopped := t.Stop()
	assert.True(stopped)
	tock(test, t, 0)

	// close channel to stop counter
	close(t.Ch)
}
