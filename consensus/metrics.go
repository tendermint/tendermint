package consensus

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Height of the chain.
	Height metrics.Gauge

	// Number of rounds.
	Rounds metrics.Gauge

	// Number of validators.
	Validators metrics.Gauge
	// Total power of all validators.
	ValidatorsPower metrics.Gauge
	// Number of validators who did not sign.
	MissingValidators metrics.Gauge
	// Total power of the missing validators.
	MissingValidatorsPower metrics.Gauge
	// Number of validators who tried to double sign.
	ByzantineValidators metrics.Gauge
	// Total power of the byzantine validators.
	ByzantineValidatorsPower metrics.Gauge

	// Time between this and the last block.
	BlockIntervalSeconds metrics.Gauge

	// Number of transactions.
	NumTxs metrics.Gauge
	// Size of the block.
	BlockSizeBytes metrics.Gauge
	// Total number of transactions.
	TotalTxs metrics.Gauge
}

// PrometheusMetrics returns Metrics build using Prometheus client library.
func PrometheusMetrics() *Metrics {
	return &Metrics{
		Height: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "height",
			Help:      "Height of the chain.",
		}, []string{}),
		Rounds: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "rounds",
			Help:      "Number of rounds.",
		}, []string{}),

		Validators: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "validators",
			Help:      "Number of validators.",
		}, []string{}),
		ValidatorsPower: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "validators_power",
			Help:      "Total power of all validators.",
		}, []string{}),
		MissingValidators: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "missing_validators",
			Help:      "Number of validators who did not sign.",
		}, []string{}),
		MissingValidatorsPower: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "missing_validators_power",
			Help:      "Total power of the missing validators.",
		}, []string{}),
		ByzantineValidators: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "byzantine_validators",
			Help:      "Number of validators who tried to double sign.",
		}, []string{}),
		ByzantineValidatorsPower: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "byzantine_validators_power",
			Help:      "Total power of the byzantine validators.",
		}, []string{}),

		BlockIntervalSeconds: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "block_interval_seconds",
			Help:      "Time between this and the last block.",
		}, []string{}),

		NumTxs: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "num_txs",
			Help:      "Number of transactions.",
		}, []string{}),
		BlockSizeBytes: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "block_size_bytes",
			Help:      "Size of the block.",
		}, []string{}),
		TotalTxs: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "consensus",
			Name:      "total_txs",
			Help:      "Total number of transactions.",
		}, []string{}),
	}
}

// NopMetrics returns no-op Metrics.
func NopMetrics() *Metrics {
	return &Metrics{
		Height: discard.NewGauge(),

		Rounds: discard.NewGauge(),

		Validators:               discard.NewGauge(),
		ValidatorsPower:          discard.NewGauge(),
		MissingValidators:        discard.NewGauge(),
		MissingValidatorsPower:   discard.NewGauge(),
		ByzantineValidators:      discard.NewGauge(),
		ByzantineValidatorsPower: discard.NewGauge(),

		BlockIntervalSeconds: discard.NewGauge(),

		NumTxs:         discard.NewGauge(),
		BlockSizeBytes: discard.NewGauge(),
		TotalTxs:       discard.NewGauge(),
	}
}
