package common

import (
	"sync"
	"time"
)

/* Debouncer */
type Debouncer struct {
	Ch    chan struct{}
	quit  chan struct{}
	dur   time.Duration
	mtx   sync.Mutex
	timer *time.Timer
}

func NewDebouncer(dur time.Duration) *Debouncer {
	var timer *time.Timer
	var ch = make(chan struct{})
	var quit = make(chan struct{})
	var mtx sync.Mutex
	fire := func() {
		go func() {
			select {
			case ch <- struct{}{}:
			case <-quit:
			}
		}()
		mtx.Lock()
		defer mtx.Unlock()
		timer.Reset(dur)
	}
	timer = time.AfterFunc(dur, fire)
	return &Debouncer{Ch: ch, dur: dur, quit: quit, mtx: mtx, timer: timer}
}

func (d *Debouncer) Reset() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.timer.Reset(d.dur)
}

func (d *Debouncer) Stop() bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	close(d.quit)
	return d.timer.Stop()
}
