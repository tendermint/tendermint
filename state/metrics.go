package state

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const MetricsSubsystem = "state"

type Metrics struct {
	// Time between BeginBlock and EndBlock.
	BlockProcessingTime metrics.Histogram
}

func PrometheusMetrics(namespace string) *Metrics {
	return &Metrics{
		BlockProcessingTime: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "block_processing_time",
			Help:      "Time between BeginBlock and EndBlock in ms.",
			Buckets:   stdprometheus.LinearBuckets(1, 10, 10),
		}, []string{}),
	}
}

func NopMetrics() *Metrics {
	return &Metrics{
		BlockProcessingTime: discard.NewHistogram(),
	}
}
