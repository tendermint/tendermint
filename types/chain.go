package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	. "github.com/tendermint/go-common"
	tmtypes "github.com/tendermint/tendermint/types"
)

// waitign more than this many seconds for a block means we're unhealthy
const newBlockTimeoutSeconds = 5

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

func (cs *ChainState) SetOnline(val *ValidatorState, isOnline bool) {
	cs.Status.SetOnline(val, isOnline)
}

func (cs *ChainState) ReconnectValidator(val *ValidatorState) {
	cs.Status.ReconnectValidator(val)
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
	Height         int     `json:"height"` // latest height we've got
	BlockchainSize int64   `json:"blockchain_size"`
	MeanBlockTime  float64 `json:"mean_block_time" wire:"unsafe"` // ms (avg over last minute)
	TxThroughput   float64 `json:"tx_throughput" wire:"unsafe"`   // tx/s (avg over last minute)

	blockTimeMeter    metrics.Meter
	txThroughputMeter metrics.Meter

	// Network Info
	NumValidators    int `json:"num_validators"`
	ActiveValidators int `json:"active_validators"`
	//ActiveNodes      int     `json:"active_nodes"`
	MeanLatency float64 `json:"mean_latency" wire:"unsafe"` // ms

	// Health
	FullHealth bool `json:"full_health"` // all validators online, synced, making blocks
	Healthy    bool `json:"healthy"`     // we're making blocks

	// Uptime
	UptimeData *UptimeData `json:"uptime_data"`

	// What else can we get / do we want?
	// TODO: charts for block time, latency (websockets/event-meter ?)
}

type UptimeData struct {
	StartTime time.Time `json:"start_time"`
	Uptime    float64   `json:"uptime" wire:"unsafe"` // Percentage of time we've been Healthy, ever

	totalDownTime time.Duration // total downtime (only updated when we come back online)
	wentDown      time.Time

	// TODO: uptime over last day, month, year
}

func NewBlockchainStatus() *BlockchainStatus {
	return &BlockchainStatus{
		blockTimeMeter:    metrics.NewMeter(),
		txThroughputMeter: metrics.NewMeter(),
		Healthy:           true,
		UptimeData: &UptimeData{
			StartTime: time.Now(),
			Uptime:    100.0,
		},
	}
}

func (s *BlockchainStatus) NewBlock(block *tmtypes.Block) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if block.Header.Height > s.Height {
		s.Height = block.Header.Height
		s.blockTimeMeter.Mark(1)
		s.txThroughputMeter.Mark(int64(block.Header.NumTxs))
		s.MeanBlockTime = (1 / s.blockTimeMeter.Rate1()) * 1000 // 1/s to ms
		s.TxThroughput = s.txThroughputMeter.Rate1()

		// if we're making blocks, we're healthy
		if !s.Healthy {
			s.Healthy = true
			s.UptimeData.totalDownTime += time.Since(s.UptimeData.wentDown)
		}

		// if we are connected to all validators, we're at full health
		// TODO: make sure they're all at the same height (within a block) and all proposing (and possibly validating )
		// Alternatively, just check there hasn't been a new round in numValidators rounds
		if s.ActiveValidators == s.NumValidators {
			s.FullHealth = true
		}

		// TODO: should we refactor so there's a central loop and ticker?
		go s.newBlockTimeout(s.Height)
	}
}

// we have newBlockTimeoutSeconds to make a new block, else we're unhealthy
func (s *BlockchainStatus) newBlockTimeout(height int) {
	time.Sleep(time.Second * newBlockTimeoutSeconds)

	s.mtx.Lock()
	defer s.mtx.Unlock()
	if !(s.Height > height) {
		s.Healthy = false
		s.UptimeData.wentDown = time.Now()
	}
}

// Used to calculate uptime on demand. TODO: refactor this into the central loop ...
func (s *BlockchainStatus) RealTimeUpdates() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	since := time.Since(s.UptimeData.StartTime)
	uptime := since - s.UptimeData.totalDownTime
	if !s.Healthy {
		uptime -= time.Since(s.UptimeData.wentDown)
	}
	s.UptimeData.Uptime = float64(uptime) / float64(since)
}

func (s *BlockchainStatus) UpdateLatency(oldLatency, newLatency float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// update avg validator rpc latency
	mean := s.MeanLatency * float64(s.NumValidators)
	mean = (mean - oldLatency + newLatency) / float64(s.NumValidators)
	s.MeanLatency = mean
}

// Toggle validators online/offline (updates ActiveValidators and FullHealth)
func (s *BlockchainStatus) SetOnline(val *ValidatorState, isOnline bool) {
	val.SetOnline(isOnline)

	var change int
	if isOnline {
		change = 1
	} else {
		change = -1
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.ActiveValidators += change

	if s.ActiveValidators > s.NumValidators {
		panic(Fmt("got %d validators. max %ds", s.ActiveValidators, s.NumValidators))
	}

	// if we lost a connection we're no longer at full health, even if it's still online.
	// so long as we receive blocks, we'll know we're still healthy
	if s.ActiveValidators != s.NumValidators {
		s.FullHealth = false
	}
}

// called in a go routine
func (s *BlockchainStatus) ReconnectValidator(val *ValidatorState) {
	for {

	}
}

func TwoThirdsMaj(count, total int) bool {
	return float64(count) > (2.0/3.0)*float64(total)
}
