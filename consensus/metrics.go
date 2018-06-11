package consensus

import "github.com/go-kit/kit/metrics"
import "github.com/go-kit/kit/metrics/discard"

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// height of the chain
	Height metrics.Counter
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Height: discard.NewCounter(),
	}
}
