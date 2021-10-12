package proxy

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this
	// package.
	MetricsSubsystem = "abci_connection"
)

type Metrics struct {
	MethodTiming metrics.Histogram
}

func PrometheusMetrics(namespace string, defaultLabelsAndValues ...string) *Metrics {
	defaultLabels := []string{}
	for i := 0; i < len(defaultLabelsAndValues); i += 2 {
		defaultLabels = append(defaultLabels, defaultLabelsAndValues[i])
	}
	return &Metrics{
		MethodTiming: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "method_timing",
			Help:      "ABCI Method Timing",
		}, append(defaultLabels, []string{"method", "type"}...)).With(defaultLabelsAndValues...),
	}
}

func NopMetrics() *Metrics {
	return &Metrics{
		MethodTiming: discard.NewHistogram(),
	}
}
