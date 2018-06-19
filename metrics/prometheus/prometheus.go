package prometheus

import (
	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	cs "github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
)

// Consensus returns consensus Metrics build using Prometheus client library.
func Consensus() *cs.Metrics {
	return &cs.Metrics{
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

		BlockIntervalSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Subsystem: "consensus",
			Name:      "block_interval_seconds",
			Help:      "Time between this and the last block.",
			Buckets:   []float64{1, 2.5, 5, 10, 60},
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

// P2P returns p2p Metrics build using Prometheus client library.
func P2P() *p2p.Metrics {
	return &p2p.Metrics{
		Peers: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "p2p",
			Name:      "peers",
			Help:      "Number of peers.",
		}, []string{}),
	}
}

// Mempool returns mempool Metrics build using Prometheus client library.
func Mempool() *mempl.Metrics {
	return &mempl.Metrics{
		Size: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Subsystem: "mempool",
			Name:      "size",
			Help:      "Size of the mempool (number of uncommitted transactions).",
		}, []string{}),
	}
}
