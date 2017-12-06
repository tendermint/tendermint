package trust

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrustMetricScores(t *testing.T) {
	tm := NewMetric()

	// Perfect score
	tm.GoodEvents(1)
	score := tm.TrustScore()
	assert.Equal(t, 100, score)

	// Less than perfect score
	tm.BadEvents(10)
	score = tm.TrustScore()
	assert.NotEqual(t, 100, score)
	tm.Stop()
}

func TestTrustMetricConfig(t *testing.T) {
	// 7 days
	window := time.Minute * 60 * 24 * 7
	config := TrustMetricConfig{
		TrackingWindow: window,
		IntervalLength: 2 * time.Minute,
	}

	tm := NewMetricWithConfig(config)

	// The max time intervals should be the TrackingWindow / IntervalLen
	assert.Equal(t, int(config.TrackingWindow/config.IntervalLength), tm.maxIntervals)

	dc := DefaultConfig()
	// These weights should still be the default values
	assert.Equal(t, dc.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, dc.IntegralWeight, tm.integralWeight)
	tm.Stop()

	config.ProportionalWeight = 0.3
	config.IntegralWeight = 0.7
	tm = NewMetricWithConfig(config)

	// These weights should be equal to our custom values
	assert.Equal(t, config.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, config.IntegralWeight, tm.integralWeight)
	tm.Stop()
}

func TestTrustMetricStopPause(t *testing.T) {
	// Cause time intervals to pass quickly
	config := TrustMetricConfig{
		TrackingWindow: 5 * time.Minute,
		IntervalLength: 10 * time.Millisecond,
	}

	tm := NewMetricWithConfig(config)

	// Allow some time intervals to pass and pause
	time.Sleep(50 * time.Millisecond)
	tm.Pause()
	// Give the pause some time to take place
	time.Sleep(10 * time.Millisecond)

	first := tm.Copy().numIntervals
	// Allow more time to pass and check the intervals are unchanged
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, first, tm.numIntervals)

	// Get the trust metric activated again
	tm.GoodEvents(5)
	// Allow some time intervals to pass and stop
	time.Sleep(50 * time.Millisecond)
	tm.Stop()
	// Give the stop some time to take place
	time.Sleep(10 * time.Millisecond)

	second := tm.Copy().numIntervals
	// Allow more time to pass and check the intervals are unchanged
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, second, tm.numIntervals)

	if first >= second {
		t.Fatalf("numIntervals should always increase or stay the same over time")
	}
}
