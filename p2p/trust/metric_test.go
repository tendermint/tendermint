package trust

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrustMetricScores(t *testing.T) {
	tm := NewMetric()
	tm.Start()

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
	tm.Start()

	// The max time intervals should be the TrackingWindow / IntervalLen
	assert.Equal(t, int(config.TrackingWindow/config.IntervalLength), tm.maxIntervals)

	dc := DefaultConfig()
	// These weights should still be the default values
	assert.Equal(t, dc.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, dc.IntegralWeight, tm.integralWeight)
	tm.Stop()
	tm.Wait()

	config.ProportionalWeight = 0.3
	config.IntegralWeight = 0.7
	tm = NewMetricWithConfig(config)
	tm.Start()

	// These weights should be equal to our custom values
	assert.Equal(t, config.ProportionalWeight, tm.proportionalWeight)
	assert.Equal(t, config.IntegralWeight, tm.integralWeight)
	tm.Stop()
	tm.Wait()
}

func TestTrustMetricCopyNilPointer(t *testing.T) {
	var tm *TrustMetric

	ctm := tm.Copy()

	assert.Nil(t, ctm)
}

// XXX: This test fails non-deterministically
//nolint:unused,deadcode
func _TestTrustMetricStopPause(t *testing.T) {
	// The TestTicker will provide manual control over
	// the passing of time within the metric
	tt := NewTestTicker()
	tm := NewMetric()
	tm.SetTicker(tt)
	tm.Start()
	// Allow some time intervals to pass and pause
	tt.NextTick()
	tt.NextTick()
	tm.Pause()

	// could be 1 or 2 because Pause and NextTick race
	first := tm.Copy().numIntervals

	// Allow more time to pass and check the intervals are unchanged
	tt.NextTick()
	tt.NextTick()
	assert.Equal(t, first, tm.Copy().numIntervals)

	// Get the trust metric activated again
	tm.GoodEvents(5)
	// Allow some time intervals to pass and stop
	tt.NextTick()
	tt.NextTick()
	tm.Stop()
	tm.Wait()

	second := tm.Copy().numIntervals
	// Allow more intervals to pass while the metric is stopped
	// and check that the number of intervals match
	tm.NextTimeInterval()
	tm.NextTimeInterval()
	// XXX: fails non-deterministically:
	// expected 5, got 6
	assert.Equal(t, second+2, tm.Copy().numIntervals)

	if first > second {
		t.Fatalf("numIntervals should always increase or stay the same over time")
	}
}
