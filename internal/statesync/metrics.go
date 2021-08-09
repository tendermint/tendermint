package statesync

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this package.
	MetricsSubsystem = "statesync"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	TotalSnapshots     metrics.Counter
	ChunkProcess       metrics.Gauge
	SnapshotHeight     metrics.Gauge
	SnapshotChunk      metrics.Counter
	SnapshotChunkTotal metrics.Gauge
	BackFill           metrics.Counter
	BackFillTotal      metrics.Gauge
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
		TotalSnapshots: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_total_snapshots",
			Help:      "The total number of snapshots discovered.",
		}, labels).With(labelsAndValues...),
		ChunkProcess: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_chunk_process",
			Help:      "The average processing time per chunk.",
		}, labels).With(labelsAndValues...),
		SnapshotHeight: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_snapshot_height",
			Help:      "The height of the current snapshot the has been processed.",
		}, labels).With(labelsAndValues...),
		SnapshotChunk: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_snapshot_chunk",
			Help:      "The current number of chunks that have been processed.",
		}, labels).With(labelsAndValues...),
		SnapshotChunkTotal: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_snapshot_chunks_total",
			Help:      "The total number of chunks in the current snapshot.",
		}, labels).With(labelsAndValues...),
		BackFill: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_backfill",
			Help:      "The current number of blocks that have been back-filled.",
		}, labels).With(labelsAndValues...),
		BackFillTotal: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "statesync_backfill_total",
			Help:      "The total number of blocks that need to be back-filled.",
		}, labels).With(labelsAndValues...),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		TotalSnapshots:     discard.NewCounter(),
		ChunkProcess:       discard.NewGauge(),
		SnapshotHeight:     discard.NewGauge(),
		SnapshotChunk:      discard.NewCounter(),
		SnapshotChunkTotal: discard.NewGauge(),
		BackFill:           discard.NewCounter(),
		BackFillTotal:      discard.NewGauge(),
	}
}
