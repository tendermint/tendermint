package mempool

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const MetricsSubsytem = "mempool"

// Metrics contains metrics exposed by this package.
// see MetricsProvider for descriptions.
type Metrics struct {
	// Size of the mempool.
	Size metrics.Gauge
	// Histogram of transaction sizes, in bytes.
	TxSizeBytes metrics.Histogram
	// Number of failed transactions.
	FailedTxs metrics.Counter
	// Number of times transactions are rechecked in the mempool.
	RecheckTimes metrics.Counter
}

// PrometheusMetrics returns Metrics build using Prometheus client library.
func PrometheusMetrics(namespace string) *Metrics {
	return &Metrics{
		Size: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsytem,
			Name:      "size",
			Help:      "Size of the mempool (number of uncommitted transactions).",
		}, []string{}),
		TxSizeBytes: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsytem,
			Name:      "tx_size_bytes",
			Help:      "Transaction sizes in bytes.",
			Buckets:   stdprometheus.ExponentialBuckets(1, 3, 17),
		}, []string{}),
		FailedTxs: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsytem,
			Name:      "failed_txs",
			Help:      "Number of failed transactions.",
		}, []string{}),
		RecheckTimes: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsytem,
			Name:      "recheck_times",
			Help:      "Number of times transactions are rechecked in the mempool.",
		}, []string{}),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Size:         discard.NewGauge(),
		TxSizeBytes:  discard.NewHistogram(),
		FailedTxs:    discard.NewCounter(),
		RecheckTimes: discard.NewCounter(),
	}
}
