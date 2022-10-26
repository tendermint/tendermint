package p2p

import (
	"github.com/go-kit/kit/metrics"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this
	// package.
	MetricsSubsystem = "p2p"
)

//go:generate go run ../scripts/metricsgen -struct=Metrics

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Number of peers.
	Peers metrics.Gauge
	// Number of bytes received from a given peer.
	PeerReceiveBytesTotal metrics.Counter `metrics_labels:"peer_id,chID"`
	// Number of bytes sent to a given peer.
	PeerSendBytesTotal metrics.Counter `metrics_labels:"peer_id,chID"`
	// Pending bytes to be sent to a given peer.
	PeerPendingSendBytes metrics.Gauge `metrics_labels:"peer_id"`
	// Number of transactions submitted by each peer.
	NumTxs metrics.Gauge `metrics_labels:"peer_id"`
}
