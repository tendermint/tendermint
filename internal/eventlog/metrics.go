package eventlog

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// gauge is the subset of the Prometheus gauge interface used here.
type gauge interface {
	Set(float64)
}

// Metrics define the metrics exported by the eventlog package.
type Metrics struct {
	numItemsGauge gauge
}

// discard is a no-op implementation of the gauge interface.
type discard struct{}

func (discard) Set(float64) {}

const eventlogSubsystem = "eventlog"

// PrometheusMetrics returns a collection of eventlog metrics for Prometheus.
func PrometheusMetrics(ns string, fields ...string) *Metrics {
	var labels []string
	for i := 0; i < len(fields); i += 2 {
		labels = append(labels, fields[i])
	}
	return &Metrics{
		numItemsGauge: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: eventlogSubsystem,
			Name:      "num_items",
			Help:      "Number of items currently resident in the event log.",
		}, labels).With(fields...),
	}
}
