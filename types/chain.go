package types

import (
	"fmt"
	"sync"

	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
	tmtypes "github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/tendermint/types"
)

//------------------------------------------------
// blockchain types
//------------------------------------------------

// Known chain and validator set IDs (from which anything else can be found)
// Returned by the Status RPC
type ChainAndValidatorSetIDs struct {
	ChainIDs        []string `json:"chain_ids"`
	ValidatorSetIDs []string `json:"validator_set_ids"`
}

//------------------------------------------------
// chain state

// Main chain state
// Returned over RPC; also used to manage state
type ChainState struct {
	Config *BlockchainConfig `json:"config"`
	Status *BlockchainStatus `json:"status"`
}

func (cs *ChainState) NewBlock(block *tmtypes.Block) {
	cs.Status.NewBlock(block)
}

func (cs *ChainState) UpdateLatency(oldLatency, newLatency float64) {
	cs.Status.UpdateLatency(oldLatency, newLatency)
}

//------------------------------------------------
// Blockchain Config: id, validator config

// Chain Config
type BlockchainConfig struct {
	// should be fixed for life of chain
	ID       string `json:"id"`
	ValSetID string `json:"val_set_id"` // NOTE: do we really commit to one val set per chain?

	// handles live validator states (latency, last block, etc)
	// and validator set changes
	mtx        sync.Mutex
	Validators []*ValidatorState `json:"validators"` // TODO: this should be ValidatorConfig and the state in BlockchainStatus
	valIDMap   map[string]int    // map IDs to indices
}

// So we can fetch validator by id rather than index
func (bc *BlockchainConfig) PopulateValIDMap() {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	bc.valIDMap = make(map[string]int)
	for i, v := range bc.Validators {
		bc.valIDMap[v.Config.Validator.ID] = i
	}
}

func (bc *BlockchainConfig) GetValidatorByID(valID string) (*ValidatorState, error) {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	valIndex, ok := bc.valIDMap[valID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator %s", valID)
	}
	return bc.Validators[valIndex], nil
}

//------------------------------------------------
// BlockchainStatus

// Basic blockchain metrics
type BlockchainStatus struct {
	mtx sync.Mutex

	// Blockchain Info
	Height         int     `json:"height"`
	BlockchainSize int64   `json:"blockchain_size"`
	MeanBlockTime  float64 `json:"mean_block_time" wire:"unsafe"`
	TxThroughput   float64 `json:"tx_throughput" wire:"unsafe"`

	blockTimeMeter    metrics.Meter
	txThroughputMeter metrics.Meter

	// Network Info
	NumValidators    int     `json:"num_validators"`
	ActiveValidators int     `json:"active_validators"`
	ActiveNodes      int     `json:"active_nodes"`
	MeanLatency      float64 `json:"mean_latency" wire:"unsafe"`
	Uptime           float64 `json:"uptime" wire:"unsafe"`

	// What else can we get / do we want?
	// TODO: charts for block time, latency (websockets/event-meter ?)
}

func NewBlockchainStatus() *BlockchainStatus {
	return &BlockchainStatus{
		blockTimeMeter:    metrics.NewMeter(),
		txThroughputMeter: metrics.NewMeter(),
	}
}

func (s *BlockchainStatus) NewBlock(block *tmtypes.Block) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if block.Header.Height > s.Height {
		s.Height = block.Header.Height
		s.blockTimeMeter.Mark(1)
		s.txThroughputMeter.Mark(int64(block.Header.NumTxs))
		s.MeanBlockTime = 1 / s.blockTimeMeter.RateMean()
		s.TxThroughput = s.txThroughputMeter.RateMean()
	}
}

func (s *BlockchainStatus) UpdateLatency(oldLatency, newLatency float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// update latency for this validator and avg latency for chain
	mean := s.MeanLatency * float64(s.NumValidators)
	mean = (mean - oldLatency + newLatency) / float64(s.NumValidators)
	s.MeanLatency = mean

	// TODO: possibly update active nodes and uptime for chain
	s.ActiveValidators = s.NumValidators // XXX
}
