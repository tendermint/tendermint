package trust

import "time"

// TrustMetricConfig - Configures the weight functions and time intervals for the metric
type TrustMetricConfig struct {
	// Determines the percentage given to current behavior
	ProportionalWeight float64

	// Determines the percentage given to prior behavior
	IntegralWeight float64

	// The window of time that the trust metric will track events across.
	// This can be set to cover many days without issue
	TrackingWindow time.Duration

	// Each interval should be short for adapability.
	// Less than 30 seconds is too sensitive,
	// and greater than 5 minutes will make the metric numb
	IntervalLength time.Duration
}

// DefaultConfig returns a config with values that have been tested and produce desirable results
func DefaultConfig() TrustMetricConfig {
	return TrustMetricConfig{
		ProportionalWeight: 0.4,
		IntegralWeight:     0.6,
		TrackingWindow:     (time.Minute * 60 * 24) * 14, // 14 days.
		IntervalLength:     1 * time.Minute,
	}
}

// Ensures that all configuration elements have valid values
func customConfig(tmc TrustMetricConfig) TrustMetricConfig {
	config := DefaultConfig()

	// Check the config for set values, and setup appropriately
	if tmc.ProportionalWeight > 0 {
		config.ProportionalWeight = tmc.ProportionalWeight
	}

	if tmc.IntegralWeight > 0 {
		config.IntegralWeight = tmc.IntegralWeight
	}

	if tmc.IntervalLength > time.Duration(0) {
		config.IntervalLength = tmc.IntervalLength
	}

	if tmc.TrackingWindow > time.Duration(0) &&
		tmc.TrackingWindow >= config.IntervalLength {
		config.TrackingWindow = tmc.TrackingWindow
	}
	return config
}
