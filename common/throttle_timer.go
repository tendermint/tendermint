package common

import (
	"sync/atomic"
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
	quit  chan struct{}
	dur   time.Duration
	timer *time.Timer
	isSet uint32
}

func NewThrottleTimer(name string, dur time.Duration) *ThrottleTimer {
	var ch = make(chan struct{})
	var quit = make(chan struct{})
	var t = &ThrottleTimer{Name: name, Ch: ch, dur: dur, quit: quit}
	t.timer = time.AfterFunc(dur, t.fireRoutine)
	t.timer.Stop()
	return t
}

func (t *ThrottleTimer) fireRoutine() {
	select {
	case t.Ch <- struct{}{}:
		atomic.StoreUint32(&t.isSet, 0)
	case <-t.quit:
		// do nothing
	default:
		t.timer.Reset(t.dur)
	}
}

func (t *ThrottleTimer) Set() {
	if atomic.CompareAndSwapUint32(&t.isSet, 0, 1) {
		t.timer.Reset(t.dur)
	}
}

func (t *ThrottleTimer) Stop() bool {
	close(t.quit)
	return t.timer.Stop()
}
