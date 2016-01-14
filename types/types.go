package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-event-meter"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//---------------------------------------------
// core types

// Known chain and validator set IDs (from which anything else can be found)
type ChainAndValidatorIDs struct {
	ChainIDs        []string `json:"chain_ids"`
	ValidatorSetIDs []string `json:"validator_set_ids"`
}

// state of a chain
type ChainStatus struct {
	Config *BlockchainConfig `json:"config"`
	Status *BlockchainStatus `json:"status"`
}

// basic chain config
// threadsafe
type BlockchainConfig struct {
	mtx sync.Mutex

	ID         string            `json:"id"`
	ValSetID   string            `json:"val_set_id"`
	Validators []*ChainValidator `json:"validators"`
	valIDMap   map[string]int    // map IDs to indices
}

func (bc *BlockchainConfig) PopulateValIDMap() {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	bc.valIDMap = make(map[string]int)
	for i, v := range bc.Validators {
		bc.valIDMap[v.ID] = i
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

// basic chain status/metrics
// threadsafe
type BlockchainStatus struct {
	mtx sync.Mutex

	Height        int     `json:"height"`
	MeanBlockTime float64 `json:"mean_block_time" wire:"unsafe"`
	TxThroughput  float64 `json:"tx_throughput" wire:"unsafe"`

	BlockchainSize int64 `json:"blockchain_size"` // how might we get StateSize ?
}

// validator on a chain
type ChainValidator struct {
	*Validator `json:"validator"`
	Addr       string `json:"addr"` // do we want multiple addrs?
	Index      int    `json:"index"`

	em      *eventmeter.EventMeter // holds a ws connection to the val
	Latency float64                `json:"latency,omitempty" wire:"unsafe"`
}

func unmarshalEvent(b json.RawMessage) (string, events.EventData, error) {
	var err error
	result := new(ctypes.TMResult)
	wire.ReadJSONPtr(result, b, &err)
	if err != nil {
		return "", nil, err
	}
	event, ok := (*result).(*ctypes.ResultEvent)
	if !ok {
		return "", nil, fmt.Errorf("Result is not type *ctypes.ResultEvent. Got %v", reflect.TypeOf(*result))
	}
	return event.Name, event.Data, nil

}

func (cv *ChainValidator) NewEventMeter() error {
	em := eventmeter.NewEventMeter(fmt.Sprintf("ws://%s/websocket", cv.Addr), unmarshalEvent)
	if err := em.Start(); err != nil {
		return err
	}
	cv.em = em
	return nil
}

func (cv *ChainValidator) EventMeter() *eventmeter.EventMeter {
	return cv.em
}

// validator set (independent of chains)
type ValidatorSet struct {
	Validators []*Validator `json:"validators"`
}

func (vs *ValidatorSet) Validator(valID string) (*Validator, error) {
	for _, v := range vs.Validators {
		if v.ID == valID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Unknwon validator %s", valID)
}

// validator (independent of chain)
type Validator struct {
	ID     string         `json:"id"`
	PubKey crypto.PubKey  `json:"pub_key,omitempty"`
	Chains []*ChainStatus `json:"chains,omitempty"`
}

func (v *Validator) Chain(chainID string) (*ChainStatus, error) {
	for _, c := range v.Chains {
		if c.Config.ID == chainID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("Unknwon chain %s", chainID)
}
