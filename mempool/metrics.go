package mempool

import "github.com/go-kit/kit/metrics"
import "github.com/go-kit/kit/metrics/discard"

// Metrics contains metrics exposed by this package.
// see MetricsProvider for descriptions.
type Metrics struct {
	Size metrics.Gauge
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Size: discard.NewGauge(),
	}
}
