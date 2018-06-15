package consensus

import "github.com/go-kit/kit/metrics"
import "github.com/go-kit/kit/metrics/discard"

// Metrics contains metrics exposed by this package.
// see MetricsProvider for descriptions.
type Metrics struct {
	Height metrics.Gauge

	Rounds metrics.Gauge

	Validators               metrics.Gauge
	ValidatorsPower          metrics.Gauge
	MissingValidators        metrics.Gauge
	MissingValidatorsPower   metrics.Gauge
	ByzantineValidators      metrics.Gauge
	ByzantineValidatorsPower metrics.Gauge

	BlockIntervalSeconds metrics.Histogram

	NumTxs         metrics.Gauge
	BlockSizeBytes metrics.Gauge
	TotalTxs       metrics.Gauge
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Height: discard.NewGauge(),

		Rounds: discard.NewGauge(),

		Validators:               discard.NewGauge(),
		ValidatorsPower:          discard.NewGauge(),
		MissingValidators:        discard.NewGauge(),
		MissingValidatorsPower:   discard.NewGauge(),
		ByzantineValidators:      discard.NewGauge(),
		ByzantineValidatorsPower: discard.NewGauge(),

		BlockIntervalSeconds: discard.NewHistogram(),

		NumTxs:         discard.NewGauge(),
		BlockSizeBytes: discard.NewGauge(),
		TotalTxs:       discard.NewGauge(),
	}
}
