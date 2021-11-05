package indexer

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// MetricsSubsystem is a the subsystem label for the indexer package.
const MetricsSubsystem = "indexer"

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Latency for indexing block events.
	BlockEventsSeconds metrics.Histogram

	// Latency for indexing transaction events.
	TxEventsSeconds metrics.Histogram

	// Number of complete blocks indexed.
	BlocksIndexed metrics.Counter

	// Number of transactions indexed.
	TransactionsIndexed metrics.Counter
}

// PrometheusMetrics returns Metrics build using Prometheus client library.
// Optionally, labels can be provided along with their values ("foo",
// "fooValue").
func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics {
	labels := []string{}
	for i := 0; i < len(labelsAndValues); i += 2 {
		labels = append(labels, labelsAndValues[i])
	}
	return &Metrics{
		BlockEventsSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "block_events_seconds",
			Help:      "Latency for indexing block events.",
		}, labels).With(labelsAndValues...),
		TxEventsSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "tx_events_seconds",
			Help:      "Latency for indexing transaction events.",
		}, labels).With(labelsAndValues...),
		BlocksIndexed: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "blocks_indexed",
			Help:      "Number of complete blocks indexed.",
		}, labels).With(labelsAndValues...),
		TransactionsIndexed: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "transactions_indexed",
			Help:      "Number of transactions indexed.",
		}, labels).With(labelsAndValues...),
	}
}

// NopMetrics returns an indexer metrics stub that discards all samples.
func NopMetrics() *Metrics {
	return &Metrics{
		BlockEventsSeconds:  discard.NewHistogram(),
		TxEventsSeconds:     discard.NewHistogram(),
		BlocksIndexed:       discard.NewCounter(),
		TransactionsIndexed: discard.NewCounter(),
	}
}
