package common

import (
	"sync/atomic"
	"time"
)

/* Throttler */
type Throttler struct {
	Ch    chan struct{}
	quit  chan struct{}
	dur   time.Duration
	timer *time.Timer
	isSet uint32
}

func NewThrottler(dur time.Duration) *Throttler {
	var ch = make(chan struct{})
	var quit = make(chan struct{})
	var t = &Throttler{Ch: ch, dur: dur, quit: quit}
	t.timer = time.AfterFunc(dur, t.fireHandler)
	return t
}

func (t *Throttler) fireHandler() {
	select {
	case t.Ch <- struct{}{}:
		atomic.StoreUint32(&t.isSet, 0)
	case <-t.quit:
	}
}

func (t *Throttler) Set() {
	if atomic.CompareAndSwapUint32(&t.isSet, 0, 1) {
		t.timer.Reset(t.dur)
	}
}

func (t *Throttler) Stop() bool {
	close(t.quit)
	return t.timer.Stop()
}
