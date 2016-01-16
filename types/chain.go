package types

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/rcrowley/go-metrics"
	tmtypes "github.com/tendermint/tendermint/types"
)

//------------------------------------------------
// blockchain types

// Known chain and validator set IDs (from which anything else can be found)
type ChainAndValidatorSetIDs struct {
	ChainIDs        []string `json:"chain_ids"`
	ValidatorSetIDs []string `json:"validator_set_ids"`
}

// Basic chain and network metrics
type BlockchainStatus struct {
	// Blockchain Info
	Height         int     `json:"height"`
	BlockchainSize int64   `json:"blockchain_size"` // how might we get StateSize ?
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

	// TODO: charts for block time, latency (websockets/event-meter ?)
}

func NewBlockchainStatus() *BlockchainStatus {
	return &BlockchainStatus{
		blockTimeMeter:    metrics.NewMeter(),
		txThroughputMeter: metrics.NewMeter(),
	}
}

func (s *BlockchainStatus) NewBlock(block *tmtypes.Block) {
	s.Height = block.Header.Height
	s.blockTimeMeter.Mark(1)
	s.txThroughputMeter.Mark(int64(block.Header.NumTxs))
	s.MeanBlockTime = 1 / s.blockTimeMeter.RateMean()
	s.TxThroughput = s.txThroughputMeter.RateMean()
}

// Main chain state
// Returned over RPC but also used to manage state
type ChainState struct {
	Config *BlockchainConfig `json:"config"`
	Status *BlockchainStatus `json:"status"`
}

// basic chain config
// threadsafe
type BlockchainConfig struct {
	ID       string `json:"id"`
	ValSetID string `json:"val_set_id"`

	mtx        sync.Mutex
	Validators []*ChainValidator `json:"validators"`
	valIDMap   map[string]int    // map IDs to indices
}

// So we can fetch validator by id
func (bc *BlockchainConfig) PopulateValIDMap() {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	bc.valIDMap = make(map[string]int)
	for i, v := range bc.Validators {
		bc.valIDMap[v.Validator.ID] = i
	}
}

func (bc *BlockchainConfig) GetValidatorByID(valID string) (*ChainValidator, error) {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	valIndex, ok := bc.valIDMap[valID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator %s", valID)
	}
	return bc.Validators[valIndex], nil
}

func LoadChainFromFile(configFile string) (*BlockchainConfig, error) {

	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// for now we start with one blockchain loaded from file;
	// eventually more can be uploaded or created through endpoints
	chainConfig := new(BlockchainConfig)
	if err := json.Unmarshal(b, chainConfig); err != nil {
		return nil, err
	}

	return chainConfig, nil
}
