package handlers

import (
	"fmt"
	"sort"
	"sync"

	"github.com/tendermint/go-event-meter"
	"github.com/tendermint/go-wire"

	"github.com/tendermint/netmon/types"
)

type NetMonResult interface {
}

// for wire.readReflect
var _ = wire.RegisterInterface(
	struct{ NetMonResult }{},
	wire.ConcreteType{&types.ChainAndValidatorIDs{}, 0x01},
	wire.ConcreteType{&types.ChainState{}, 0x02},
	wire.ConcreteType{&types.Validator{}, 0x03},
	wire.ConcreteType{&eventmeter.EventMetric{}, 0x04},
)

//---------------------------------------------
// global state and backend functions

type TendermintNetwork struct {
	mtx     sync.Mutex
	Chains  map[string]*types.ChainState   `json:"blockchains"`
	ValSets map[string]*types.ValidatorSet `json:"validator_sets"`
}

// TODO: populate validator sets
func NewTendermintNetwork(chains ...*types.ChainState) *TendermintNetwork {
	network := &TendermintNetwork{
		Chains:  make(map[string]*types.ChainState),
		ValSets: make(map[string]*types.ValidatorSet),
	}
	for _, chain := range chains {
		network.Chains[chain.Config.ID] = chain
	}
	return network
}

//------------
// Public Methods

func (tn *TendermintNetwork) RegisterChain(chain *types.ChainState) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	tn.Chains[chain.Config.ID] = chain
}

func (tn *TendermintNetwork) Stop() {
	// TODO: for each chain, stop each validator
}

//------------
// RPC funcs

func (tn *TendermintNetwork) Status() (*types.ChainAndValidatorIDs, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	chains := make([]string, len(tn.Chains))
	valSets := make([]string, len(tn.ValSets))
	i := 0
	for chain, _ := range tn.Chains {
		chains[i] = chain
		i += 1
	}
	i = 0
	for valset, _ := range tn.ValSets {
		valSets[i] = valset
		i += 1
	}
	sort.StringSlice(chains).Sort()
	sort.StringSlice(valSets).Sort()
	return &types.ChainAndValidatorIDs{
		ChainIDs:        chains,
		ValidatorSetIDs: valSets,
	}, nil

}

func (tn *TendermintNetwork) GetChain(chainID string) (*types.ChainState, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	chain, ok := tn.Chains[chainID]
	if !ok {
		return nil, fmt.Errorf("Unknown chain %s", chainID)
	}
	fmt.Println("CHAIN:", chain)
	return chain, nil
}

func (tn *TendermintNetwork) GetValidatorSet(valSetID string) (*types.ValidatorSet, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	valSet, ok := tn.ValSets[valSetID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator set %s", valSetID)
	}
	return valSet, nil
}

func (tn *TendermintNetwork) GetValidator(valSetID, valID string) (*types.Validator, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	valSet, ok := tn.ValSets[valSetID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator set %s", valSetID)
	}
	val, err := valSet.Validator(valID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (tn *TendermintNetwork) StartMeter(chainID, valID, eventID string) error {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	val, err := tn.getChainVal(chainID, valID)
	if err != nil {
		return err
	}
	return val.EventMeter().Subscribe(eventID, nil)
}

func (tn *TendermintNetwork) StopMeter(chainID, valID, eventID string) error {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	val, err := tn.getChainVal(chainID, valID)
	if err != nil {
		return err
	}
	return val.EventMeter().Unsubscribe(eventID)
}

func (tn *TendermintNetwork) GetMeter(chainID, valID, eventID string) (*eventmeter.EventMetric, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	val, err := tn.getChainVal(chainID, valID)
	if err != nil {
		return nil, err
	}

	return val.EventMeter().GetMetric(eventID)
}

func (tn *TendermintNetwork) getChainVal(chainID, valID string) (*types.ChainValidator, error) {
	chain, ok := tn.Chains[chainID]
	if !ok {
		return nil, fmt.Errorf("Unknown chain %s", chainID)
	}
	val, err := chain.Config.GetValidatorByID(valID)
	if err != nil {
		return nil, err
	}
	return val, nil
}
