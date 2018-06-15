package consensus

import "github.com/go-kit/kit/metrics"
import "github.com/go-kit/kit/metrics/discard"

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// height of the chain
	Height metrics.Counter
	// number of validators who signed
	Validators metrics.Gauge
	// number of validators who did not sign
	MissingValidators metrics.Gauge
	// number of validators who tried to double sign
	ByzantineValidators metrics.Gauge
	// time between this and the last block
	BlockIntervalSeconds metrics.Histogram
	// number of transactions
	NumTxs metrics.Gauge
	// total number of transactions
	TotalTxs metrics.Counter
	// size of the block
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
