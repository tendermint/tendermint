package statesync

import (
	"github.com/go-kit/kit/metrics"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this package.
	MetricsSubsystem = "statesync"
)

//go:generate go run ../../scripts/metricsgen -struct=Metrics

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// The total number of snapshots discovered.
	TotalSnapshots metrics.Counter
	// The average processing time per chunk.
	ChunkProcessAvgTime metrics.Gauge
	// The height of the current snapshot the has been processed.
	SnapshotHeight metrics.Gauge
	// The current number of chunks that have been processed.
	SnapshotChunk metrics.Counter
	// The total number of chunks in the current snapshot.
	SnapshotChunkTotal metrics.Gauge
	// The current number of blocks that have been back-filled.
	BackFilledBlocks metrics.Counter
	// The total number of blocks that need to be back-filled.
	BackFillBlocksTotal metrics.Gauge
}
