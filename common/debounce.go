package common

import (
    "time"
)

/* Debouncer */
type Debouncer struct {
    Ch      chan struct{}
    quit    chan struct{}
    dur     time.Duration
    timer   *time.Timer
}

func NewDebouncer(dur time.Duration) *Debouncer {
    var timer *time.Timer
    var ch = make(chan struct{})
    var quit = make(chan struct{})
    fire := func() {
        go func() {
            select {
            case ch <- struct{}{}:
            case <-quit:
            }
        }()
        timer.Reset(dur)
    }
    timer = time.AfterFunc(dur, fire)
    return &Debouncer{Ch:ch, dur:dur, quit:quit, timer:timer}
}

func (d *Debouncer) Reset() {
    d.timer.Reset(d.dur)
}

func (d *Debouncer) Stop() bool {
    close(d.quit)
    return d.timer.Stop()
}
