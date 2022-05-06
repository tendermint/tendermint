package basic

import "github.com/go-kit/kit/metrics"

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// simple metric that tracks the height of the chain.
	Height metrics.Gauge
}
