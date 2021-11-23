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

// Metrics contains the prometheus metrics exposed by the proxy package.
type Metrics struct {
	MethodTiming metrics.Histogram
}

// PrometheusMetrics constructs a Metrics instance that collects metrics samples.
// The resulting metrics will be prefixed with namespace and labeled with the
// defaultLabelsAndValues. defaultLabelsAndValues must be a list of string pairs
// where the first of each pair is the label and the second is the value.
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
			Buckets:   []float64{.0001, .0004, .002, .009, .02, .1, .65, 2, 6, 25},
		}, append(defaultLabels, []string{"method", "type"}...)).With(defaultLabelsAndValues...),
	}
}

// NopMetrics constructs a Metrics instance that discards all samples and is suitable
// for testing.
func NopMetrics() *Metrics {
	return &Metrics{
		MethodTiming: discard.NewHistogram(),
	}
}
