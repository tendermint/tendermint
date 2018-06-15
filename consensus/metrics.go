package consensus

import "github.com/go-kit/kit/metrics"
import "github.com/go-kit/kit/metrics/discard"

// Metrics contains metrics exposed by this package.
// see MetricsProvider for descriptions.
type Metrics struct {
	Height metrics.Counter

	Validators               metrics.Gauge
	ValidatorsPower          metrics.Gauge
	MissingValidators        metrics.Gauge
	MissingValidatorsPower   metrics.Gauge
	ByzantineValidators      metrics.Gauge
	ByzantineValidatorsPower metrics.Gauge

	BlockIntervalSeconds metrics.Histogram

	NumTxs   metrics.Gauge
	TotalTxs metrics.Counter

	BlockSizeBytes metrics.Gauge
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Height:               discard.NewCounter(),
		Validators:           discard.NewGauge(),
		MissingValidators:    discard.NewGauge(),
		ByzantineValidators:  discard.NewGauge(),
		BlockIntervalSeconds: discard.NewHistogram(),
		NumTxs:               discard.NewGauge(),
		TotalTxs:             discard.NewCounter(),
		BlockSizeBytes:       discard.NewGauge(),
	}
}
