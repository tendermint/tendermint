package consensus

import (
	"strings"
	"time"

	"github.com/go-kit/kit/metrics"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/types"
)

const (
	// MetricsSubsystem is a subsystem shared by all metrics exposed by this
	// package.
	MetricsSubsystem = "consensus"
)

//go:generate go run ../scripts/metricsgen -struct=Metrics

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Height of the chain.
	Height metrics.Gauge

	// Last height signed by this validator if the node is a validator.
	ValidatorLastSignedHeight metrics.Gauge `metrics_labels:"validator_address"`

	// Number of rounds.
	Rounds metrics.Gauge

	// Histogram of round duration.
	RoundDuration metrics.Histogram `metrics_buckettype:"exprange" metrics_bucketsizes:"0.1, 100, 8"`

	// Number of validators.
	Validators metrics.Gauge
	// Total power of all validators.
	ValidatorsPower metrics.Gauge
	// Power of a validator.
	ValidatorPower metrics.Gauge `metrics_labels:"validator_address"`
	// Amount of blocks missed per validator.
	ValidatorMissedBlocks metrics.Gauge `metrics_labels:"validator_address"`
	// Number of validators who did not sign.
	MissingValidators metrics.Gauge
	// Total power of the missing validators.
	MissingValidatorsPower metrics.Gauge
	// Number of validators who tried to double sign.
	ByzantineValidators metrics.Gauge
	// Total power of the byzantine validators.
	ByzantineValidatorsPower metrics.Gauge

	// Time between this and the last block.
	BlockIntervalSeconds metrics.Histogram

	// Number of transactions.
	NumTxs metrics.Gauge
	// Size of the block.
	BlockSizeBytes metrics.Gauge
	// Total number of transactions.
	TotalTxs metrics.Gauge
	// The latest block height.
	CommittedHeight metrics.Gauge `metrics_name:"latest_block_height"`
	// Whether or not a node is fast syncing. 1 if yes, 0 if no.
	FastSyncing metrics.Gauge
	// Whether or not a node is state syncing. 1 if yes, 0 if no.
	StateSyncing metrics.Gauge

	// Number of block parts transmitted by each peer.
	BlockParts metrics.Counter `metrics_labels:"peer_id"`

	// Histogram of durations for each step in the consensus protocol.
	StepDuration metrics.Histogram `metrics_labels:"step" metrics_buckettype:"exprange" metrics_bucketsizes:"0.1, 100, 8"`
	stepStart    time.Time

	// Number of block parts received by the node, separated by whether the part
	// was relevant to the block the node is trying to gather or not.
	BlockGossipPartsReceived metrics.Counter `metrics_labels:"matches_current"`

	// QuroumPrevoteMessageDelay is the interval in seconds between the proposal
	// timestamp and the timestamp of the earliest prevote that achieved a quorum
	// during the prevote step.
	//
	// To compute it, sum the voting power over each prevote received, in increasing
	// order of timestamp. The timestamp of the first prevote to increase the sum to
	// be above 2/3 of the total voting power of the network defines the endpoint
	// the endpoint of the interval. Subtract the proposal timestamp from this endpoint
	// to obtain the quorum delay.
	//metrics:Interval in seconds between the proposal timestamp and the timestamp of the earliest prevote that achieved a quorum.
	QuorumPrevoteDelay metrics.Gauge `metrics_labels:"proposer_address"`

	// FullPrevoteDelay is the interval in seconds between the proposal
	// timestamp and the timestamp of the latest prevote in a round where 100%
	// of the voting power on the network issued prevotes.
	//metrics:Interval in seconds between the proposal timestamp and the timestamp of the latest prevote in a round where all validators voted.
	FullPrevoteDelay metrics.Gauge `metrics_labels:"proposer_address"`
}

// RecordConsMetrics uses for recording the block related metrics during fast-sync.
func (m *Metrics) RecordConsMetrics(block *types.Block) {
	m.NumTxs.Set(float64(len(block.Data.Txs)))
	m.TotalTxs.Add(float64(len(block.Data.Txs)))
	m.BlockSizeBytes.Set(float64(block.Size()))
	m.CommittedHeight.Set(float64(block.Height))
}

func (m *Metrics) MarkRound(r int32, st time.Time) {
	m.Rounds.Set(float64(r))
	roundTime := time.Since(st).Seconds()
	m.RoundDuration.Observe(roundTime)
}

func (m *Metrics) MarkStep(s cstypes.RoundStepType) {
	if !m.stepStart.IsZero() {
		stepTime := time.Since(m.stepStart).Seconds()
		stepName := strings.TrimPrefix(s.String(), "RoundStep")
		m.StepDuration.With("step", stepName).Observe(stepTime)
	}
	m.stepStart = time.Now()
}
