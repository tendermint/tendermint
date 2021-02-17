package p2p

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this
	// package.
	MetricsSubsystem = "p2p"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Number of peers.
	Peers metrics.Gauge
	// Number of bytes received from a given peer.
	PeerReceiveBytesTotal metrics.Counter
	// Number of bytes sent to a given peer.
	PeerSendBytesTotal metrics.Counter
	// Pending bytes to be sent to a given peer.
	PeerPendingSendBytes metrics.Gauge
	// Number of transactions submitted by each peer.
	NumTxs metrics.Gauge

	// RouterPeerQueueRecv defines the time taken to read off of a peer's queue
	// before sending on the connection.
	RouterPeerQueueRecv metrics.Histogram

	// RouterPeerQueueSend defines the time taken to send on a peer's queue which
	// will later be read and sent on the connection (see RouterPeerQueueRecv).
	RouterPeerQueueSend metrics.Histogram

	// RouterChannelQueueSend defines the time taken to send on a p2p channel's
	// queue which will later be consued by the corresponding reactor/service.
	RouterChannelQueueSend metrics.Histogram
}

// PrometheusMetrics returns Metrics build using Prometheus client library.
// Optionally, labels can be provided along with their values ("foo",
// "fooValue").
//
// nolint: lll
func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics {
	labels := []string{}
	for i := 0; i < len(labelsAndValues); i += 2 {
		labels = append(labels, labelsAndValues[i])
	}
	return &Metrics{
		Peers: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "peers",
			Help:      "Number of peers.",
		}, labels).With(labelsAndValues...),
		PeerReceiveBytesTotal: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "peer_receive_bytes_total",
			Help:      "Number of bytes received from a given peer.",
		}, append(labels, "peer_id", "chID")).With(labelsAndValues...),
		PeerSendBytesTotal: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "peer_send_bytes_total",
			Help:      "Number of bytes sent to a given peer.",
		}, append(labels, "peer_id", "chID")).With(labelsAndValues...),
		PeerPendingSendBytes: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "peer_pending_send_bytes",
			Help:      "Number of pending bytes to be sent to a given peer.",
		}, append(labels, "peer_id")).With(labelsAndValues...),
		NumTxs: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "num_txs",
			Help:      "Number of transactions submitted by each peer.",
		}, append(labels, "peer_id")).With(labelsAndValues...),

		RouterPeerQueueRecv: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "router_peer_queue_recv",
			Help:      "The time taken to read off of a peer's queue before sending on the connection.",
		}, labels).With(labelsAndValues...),

		RouterPeerQueueSend: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "router_peer_queue_send",
			Help:      "The time taken to send on a peer's queue which will later be read and sent on the connection (see RouterPeerQueueRecv).",
		}, labels).With(labelsAndValues...),

		RouterChannelQueueSend: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: MetricsSubsystem,
			Name:      "router_channel_queue_send",
			Help:      "The time taken to send on a p2p channel's queue which will later be consued by the corresponding reactor/service.",
		}, labels).With(labelsAndValues...),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Peers:                  discard.NewGauge(),
		PeerReceiveBytesTotal:  discard.NewCounter(),
		PeerSendBytesTotal:     discard.NewCounter(),
		PeerPendingSendBytes:   discard.NewGauge(),
		NumTxs:                 discard.NewGauge(),
		RouterPeerQueueRecv:    discard.NewHistogram(),
		RouterPeerQueueSend:    discard.NewHistogram(),
		RouterChannelQueueSend: discard.NewHistogram(),
	}
}
