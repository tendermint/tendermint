// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package trust

import (
	"time"
)

// MetricTicker provides a single ticker interface for the trust metric
type MetricTicker interface {
	// GetChannel returns the receive only channel that fires at each time interval
	GetChannel() <-chan time.Time

	// Stop will halt further activity on the ticker channel
	Stop()
}

// The ticker used during testing that provides manual control over time intervals
type TestTicker struct {
	C       chan time.Time
	stopped bool
}

// NewTestTicker returns our ticker used within test routines
func NewTestTicker() *TestTicker {
	c := make(chan time.Time)
	return &TestTicker{
		C: c,
	}
}

func (t *TestTicker) GetChannel() <-chan time.Time {
	return t.C
}

func (t *TestTicker) Stop() {
	t.stopped = true
}

// NextInterval manually sends Time on the ticker channel
func (t *TestTicker) NextTick() {
	if t.stopped {
		return
	}
	t.C <- time.Now()
}

// Ticker is just a wrap around time.Ticker that allows it
// to meet the requirements of our interface
type Ticker struct {
	*time.Ticker
}

// NewTicker returns a normal time.Ticker wrapped to meet our interface
func NewTicker(d time.Duration) *Ticker {
	return &Ticker{time.NewTicker(d)}
}

func (t *Ticker) GetChannel() <-chan time.Time {
	return t.C
}
