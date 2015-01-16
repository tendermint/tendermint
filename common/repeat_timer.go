package common

import "time"

/*
RepeatTimer repeatedly sends a struct{}{} to .Ch after each "dur" period.
It's good for keeping connections alive.
*/
type RepeatTimer struct {
	Name  string
	Ch    chan struct{}
	quit  chan struct{}
	dur   time.Duration
	timer *time.Timer
}

func NewRepeatTimer(name string, dur time.Duration) *RepeatTimer {
	var ch = make(chan struct{})
	var quit = make(chan struct{})
	var t = &RepeatTimer{Name: name, Ch: ch, dur: dur, quit: quit}
	t.timer = time.AfterFunc(dur, t.fireRoutine)
	return t
}

func (t *RepeatTimer) fireRoutine() {
	select {
	case t.Ch <- struct{}{}:
		t.timer.Reset(t.dur)
	case <-t.quit:
		// do nothing
	default:
		t.timer.Reset(t.dur)
	}
}

// Wait the duration again before firing.
func (t *RepeatTimer) Reset() {
	t.timer.Reset(t.dur)
}

func (t *RepeatTimer) Stop() bool {
	close(t.quit)
	return t.timer.Stop()
}
