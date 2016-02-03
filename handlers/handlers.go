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
	wire.ConcreteType{&types.ChainAndValidatorSetIDs{}, 0x01},
	wire.ConcreteType{&types.ChainState{}, 0x02},
	wire.ConcreteType{&types.ValidatorSet{}, 0x10},
	wire.ConcreteType{&types.Validator{}, 0x11},
	wire.ConcreteType{&types.ValidatorConfig{}, 0x12},
	wire.ConcreteType{&eventmeter.EventMetric{}, 0x20},
)

//---------------------------------------------
// global state and backend functions

// TODO: relax the locking (use RWMutex, reduce scope)
type TendermintNetwork struct {
	mtx     sync.Mutex
	Chains  map[string]*types.ChainState   `json:"blockchains"`
	ValSets map[string]*types.ValidatorSet `json:"validator_sets"`
}

func NewTendermintNetwork() *TendermintNetwork {
	network := &TendermintNetwork{
		Chains:  make(map[string]*types.ChainState),
		ValSets: make(map[string]*types.ValidatorSet),
	}
	return network
}

//------------
// Public Methods

func (tn *TendermintNetwork) Stop() {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	for _, c := range tn.Chains {
		for _, v := range c.Config.Validators {
			v.Stop()
		}
	}
}

//-----------------------------------------------------------
// RPC funcs
//-----------------------------------------------------------

//------------------
// Status

// Returns sorted lists of all chains and validator sets
func (tn *TendermintNetwork) Status() (*types.ChainAndValidatorSetIDs, error) {
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
	return &types.ChainAndValidatorSetIDs{
		ChainIDs:        chains,
		ValidatorSetIDs: valSets,
	}, nil

}

// NOTE: returned values should not be manipulated by callers as they are pointers to the state!
//------------------
// Blockchains

// Get the current state of a chain
func (tn *TendermintNetwork) GetChain(chainID string) (*types.ChainState, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	chain, ok := tn.Chains[chainID]
	if !ok {
		return nil, fmt.Errorf("Unknown chain %s", chainID)
	}
	chain.Status.RealTimeUpdates()
	return chain, nil
}

// Register a new chain on the network.
// For each validator, start a websocket connection to listen for new block events and record latency
func (tn *TendermintNetwork) RegisterChain(chainConfig *types.BlockchainConfig) (*types.ChainState, error) {
	// Don't bother locking until we touch the TendermintNetwork object

	chainState := &types.ChainState{
		Config: chainConfig,
		Status: types.NewBlockchainStatus(),
	}
	chainState.Status.NumValidators = len(chainConfig.Validators)

	// so we can easily lookup validators by id rather than index
	chainState.Config.PopulateValIDMap()

	// start the event meter and listen for new blocks on each validator
	for _, v := range chainConfig.Validators {
		v.Status = &types.ValidatorStatus{}

		if err := v.Start(); err != nil {
			return nil, fmt.Errorf("Error starting validator %s: %v", v.Config.Validator.ID, err)
		}

		// register callbacks for the validator
		tn.registerCallbacks(chainState, v)

		// the DisconnectCallback will set us offline and start a reconnect routine
		chainState.Status.SetOnline(v, true)

		// get/set the validator's pub key
		// TODO: make this authenticate...
		v.PubKey()
	}

	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	tn.Chains[chainState.Config.ID] = chainState
	return chainState, nil
}

//------------------
// Validators

func (tn *TendermintNetwork) GetValidatorSet(valSetID string) (*types.ValidatorSet, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	valSet, ok := tn.ValSets[valSetID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator set %s", valSetID)
	}
	return valSet, nil
}

func (tn *TendermintNetwork) RegisterValidatorSet(valSet *types.ValidatorSet) (*types.ValidatorSet, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	tn.ValSets[valSet.ID] = valSet
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

// Update the validator's rpc address (for now its the only thing that can be updated!)
func (tn *TendermintNetwork) UpdateValidator(chainID, valID, rpcAddr string) (*types.ValidatorConfig, error) {
	tn.mtx.Lock()
	defer tn.mtx.Unlock()
	val, err := tn.getChainVal(chainID, valID)
	if err != nil {
		return nil, err
	}

	val.Config.UpdateRPCAddress(rpcAddr)
	log.Debug("Update validator rpc address", "chain", chainID, "val", valID, "rpcAddr", rpcAddr)
	return val.Config, nil
}

//------------------
// Event metering

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

// assumes lock is held
func (tn *TendermintNetwork) getChainVal(chainID, valID string) (*types.ValidatorState, error) {
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
