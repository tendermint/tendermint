package p2p

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Number of peers.
	Peers metrics.Gauge
}

// PrometheusMetrics returns Metrics build using Prometheus client library.
func PrometheusMetrics() *Metrics {
	return &Metrics{
		Peers: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "p2p",
			Name:      "peers",
			Help:      "Number of peers.",
		}, []string{}),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Peers: discard.NewGauge(),
	}
}
