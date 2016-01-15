package types

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/tendermint/go-event-meter"
	"github.com/tendermint/go-events"
	tmtypes "github.com/tendermint/tendermint/types"
)

//------------------------------------------------
// blockchain types

// State of a chain
// Returned over RPC but also used to manage state
type ChainState struct {
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

func LoadChainFromFile(configFile string) (*ChainState, error) {

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

	chainState := &ChainState{Config: chainConfig}

	// start the event meter and listen for new blocks on each validator
	for _, v := range chainConfig.Validators {

		if err := v.Start(); err != nil {
			return nil, err
		}
		err := v.EventMeter().Subscribe(tmtypes.EventStringNewBlock(), func(metric *eventmeter.EventMetric, data events.EventData) {
			// TODO: update chain status with block and metric
			// chainState.NewBlock(data.(tmtypes.EventDataNewBlock).Block)
		})
		if err != nil {
			return nil, err
		}

		// get/set the validator's pub key
		v.PubKey()
	}
	return chainState, nil
}
