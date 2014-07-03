package common

import "time"

/* RepeatTimer */
type RepeatTimer struct {
	Ch    chan struct{}
	quit  chan struct{}
	dur   time.Duration
	timer *time.Timer
}

func NewRepeatTimer(dur time.Duration) *RepeatTimer {
	var ch = make(chan struct{})
	var quit = make(chan struct{})
	var t = &RepeatTimer{Ch: ch, dur: dur, quit: quit}
	t.timer = time.AfterFunc(dur, t.fireHandler)
	return t
}

func (t *RepeatTimer) fireHandler() {
	select {
	case t.Ch <- struct{}{}:
		t.timer.Reset(t.dur)
	case <-t.quit:
	}
}

func (t *RepeatTimer) Reset() {
	t.timer.Reset(t.dur)
}

func (t *RepeatTimer) Stop() bool {
	close(t.quit)
	return t.timer.Stop()
}
