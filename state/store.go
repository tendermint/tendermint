package state

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	// https://github.com/tendermint/tendermint/pull/3438
	// 100000 results in ~ 100ms to get 100 validators (see BenchmarkLoadValidators)
	valSetCheckpointInterval = 100000
)

//------------------------------------------------------------------------

func calcValidatorsKey(height int64) []byte {
	return []byte(fmt.Sprintf("validatorsKey:%v", height))
}

func calcConsensusParamsKey(height int64) []byte {
	return []byte(fmt.Sprintf("consensusParamsKey:%v", height))
}

func calcABCIResponsesKey(height int64) []byte {
	return []byte(fmt.Sprintf("abciResponsesKey:%v", height))
}

// LoadStateFromDBOrGenesisFile loads the most recent state from the database,
// or creates a new one from the given genesisFilePath and persists the result
// to the database.
func LoadStateFromDBOrGenesisFile(stateDB dbm.DB, genesisFilePath string) (State, error) {
	state := LoadState(stateDB)
	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisStateFromFile(genesisFilePath)
		if err != nil {
			return state, err
		}
		SaveState(stateDB, state)
	}

	return state, nil
}

// LoadStateFromDBOrGenesisDoc loads the most recent state from the database,
// or creates a new one from the given genesisDoc and persists the result
// to the database.
func LoadStateFromDBOrGenesisDoc(stateDB dbm.DB, genesisDoc *types.GenesisDoc) (State, error) {
	state := LoadState(stateDB)
	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisState(genesisDoc)
		if err != nil {
			return state, err
		}
		SaveState(stateDB, state)
	}

	return state, nil
}

// LoadState loads the State from the database.
func LoadState(db dbm.DB) State {
	return loadState(db, stateKey)
}

func loadState(db dbm.DB, key []byte) (state State) {
	buf := db.Get(key)
	if len(buf) == 0 {
		return state
	}

	err := cdc.UnmarshalBinaryBare(buf, &state)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(fmt.Sprintf(`LoadState: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return state
}

// SaveState persists the State, the ValidatorsInfo, and the ConsensusParamsInfo to the database.
// This flushes the writes (e.g. calls SetSync).
func SaveState(db dbm.DB, state State) {
	saveState(db, state, stateKey)
}

func saveState(db dbm.DB, state State, key []byte) {
	nextHeight := state.LastBlockHeight + 1
	// If first block, save validators for block 1.
	if nextHeight == 1 {
		// This extra logic due to Tendermint validator set changes being delayed 1 block.
		// It may get overwritten due to InitChain validator updates.
		lastHeightVoteChanged := int64(1)
		saveValidatorsInfo(db, nextHeight, lastHeightVoteChanged, state.Validators)
	}
	// Save next validators.
	saveValidatorsInfo(db, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	// Save next consensus params.
	saveConsensusParamsInfo(db, nextHeight, state.LastHeightConsensusParamsChanged, state.ConsensusParams)
	db.SetSync(key, state.Bytes())
}

//------------------------------------------------------------------------

// ABCIResponses retains the responses
// of the various ABCI calls during block processing.
// It is persisted to disk for each height before calling Commit.
type ABCIResponses struct {
	DeliverTx  []*abci.ResponseDeliverTx `json:"deliver_tx"`
	EndBlock   *abci.ResponseEndBlock    `json:"end_block"`
	BeginBlock *abci.ResponseBeginBlock  `json:"begin_block"`
}

// NewABCIResponses returns a new ABCIResponses
func NewABCIResponses(block *types.Block) *ABCIResponses {
	resDeliverTxs := make([]*abci.ResponseDeliverTx, block.NumTxs)
	if block.NumTxs == 0 {
		// This makes Amino encoding/decoding consistent.
		resDeliverTxs = nil
	}
	return &ABCIResponses{
		DeliverTx: resDeliverTxs,
	}
}

// Bytes serializes the ABCIResponse using go-amino.
func (arz *ABCIResponses) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(arz)
}

func (arz *ABCIResponses) ResultsHash() []byte {
	results := types.NewResults(arz.DeliverTx)
	return results.Hash()
}

// LoadABCIResponses loads the ABCIResponses for the given height from the database.
// This is useful for recovering from crashes where we called app.Commit and before we called
// s.Save(). It can also be used to produce Merkle proofs of the result of txs.
func LoadABCIResponses(db dbm.DB, height int64) (*ABCIResponses, error) {
	buf := db.Get(calcABCIResponsesKey(height))
	if len(buf) == 0 {
		return nil, ErrNoABCIResponsesForHeight{height}
	}

	abciResponses := new(ABCIResponses)
	err := cdc.UnmarshalBinaryBare(buf, abciResponses)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(fmt.Sprintf(`LoadABCIResponses: Data has been corrupted or its spec has
                changed: %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return abciResponses, nil
}

// SaveABCIResponses persists the ABCIResponses to the database.
// This is useful in case we crash after app.Commit and before s.Save().
// Responses are indexed by height so they can also be loaded later to produce Merkle proofs.
func saveABCIResponses(db dbm.DB, height int64, abciResponses *ABCIResponses) {
	db.SetSync(calcABCIResponsesKey(height), abciResponses.Bytes())
}

//-----------------------------------------------------------------------------

// ValidatorsInfo represents the latest validator set, or the last height it changed
type ValidatorsInfo struct {
	ValidatorSet      *types.ValidatorSet
	LastHeightChanged int64
}

// Bytes serializes the ValidatorsInfo using go-amino.
func (valInfo *ValidatorsInfo) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(valInfo)
}

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func LoadValidators(db dbm.DB, height int64) (*types.ValidatorSet, error) {
	valInfo := loadValidatorsInfo(db, height)
	if valInfo == nil {
		return nil, ErrNoValSetForHeight{height}
	}
	if valInfo.ValidatorSet == nil {
		lastStoredHeight := lastStoredHeightFor(height, valInfo.LastHeightChanged)
		valInfo2 := loadValidatorsInfo(db, lastStoredHeight)
		if valInfo2 == nil || valInfo2.ValidatorSet == nil {
			// TODO (melekes): remove the below if condition in the 0.33 major
			// release and just panic. Old chains might panic otherwise if they
			// haven't saved validators at intermediate (%valSetCheckpointInterval)
			// height yet.
			// https://github.com/tendermint/tendermint/issues/3543
			valInfo2 = loadValidatorsInfo(db, valInfo.LastHeightChanged)
			lastStoredHeight = valInfo.LastHeightChanged
			if valInfo2 == nil || valInfo2.ValidatorSet == nil {
				panic(
					fmt.Sprintf("Couldn't find validators at height %d (height %d was originally requested)",
						lastStoredHeight,
						height,
					),
				)
			}
		}
		valInfo2.ValidatorSet.IncrementProposerPriority(int(height - lastStoredHeight)) // mutate
		valInfo = valInfo2
	}

	return valInfo.ValidatorSet, nil
}

func lastStoredHeightFor(height, lastHeightChanged int64) int64 {
	checkpointHeight := height - height%valSetCheckpointInterval
	return cmn.MaxInt64(checkpointHeight, lastHeightChanged)
}

// CONTRACT: Returned ValidatorsInfo can be mutated.
func loadValidatorsInfo(db dbm.DB, height int64) *ValidatorsInfo {
	buf := db.Get(calcValidatorsKey(height))
	if len(buf) == 0 {
		return nil
	}

	v := new(ValidatorsInfo)
	err := cdc.UnmarshalBinaryBare(buf, v)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(fmt.Sprintf(`LoadValidators: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return v
}

// saveValidatorsInfo persists the validator set.
//
// `height` is the effective height for which the validator is responsible for
// signing. It should be called from s.Save(), right before the state itself is
// persisted.
func saveValidatorsInfo(db dbm.DB, height, lastHeightChanged int64, valSet *types.ValidatorSet) {
	if lastHeightChanged > height {
		panic("LastHeightChanged cannot be greater than ValidatorsInfo height")
	}
	valInfo := &ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}
	// Only persist validator set if it was updated or checkpoint height (see
	// valSetCheckpointInterval) is reached.
	if height == lastHeightChanged || height%valSetCheckpointInterval == 0 {
		valInfo.ValidatorSet = valSet
	}
	db.Set(calcValidatorsKey(height), valInfo.Bytes())
}

//-----------------------------------------------------------------------------

// ConsensusParamsInfo represents the latest consensus params, or the last height it changed
type ConsensusParamsInfo struct {
	ConsensusParams   types.ConsensusParams
	LastHeightChanged int64
}

// Bytes serializes the ConsensusParamsInfo using go-amino.
func (params ConsensusParamsInfo) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(params)
}

// LoadConsensusParams loads the ConsensusParams for a given height.
func LoadConsensusParams(db dbm.DB, height int64) (types.ConsensusParams, error) {
	empty := types.ConsensusParams{}

	paramsInfo := loadConsensusParamsInfo(db, height)
	if paramsInfo == nil {
		return empty, ErrNoConsensusParamsForHeight{height}
	}

	if paramsInfo.ConsensusParams.Equals(&empty) {
		paramsInfo2 := loadConsensusParamsInfo(db, paramsInfo.LastHeightChanged)
		if paramsInfo2 == nil {
			panic(
				fmt.Sprintf(
					"Couldn't find consensus params at height %d as last changed from height %d",
					paramsInfo.LastHeightChanged,
					height,
				),
			)
		}
		paramsInfo = paramsInfo2
	}

	return paramsInfo.ConsensusParams, nil
}

func loadConsensusParamsInfo(db dbm.DB, height int64) *ConsensusParamsInfo {
	buf := db.Get(calcConsensusParamsKey(height))
	if len(buf) == 0 {
		return nil
	}

	paramsInfo := new(ConsensusParamsInfo)
	err := cdc.UnmarshalBinaryBare(buf, paramsInfo)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(fmt.Sprintf(`LoadConsensusParams: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return paramsInfo
}

// saveConsensusParamsInfo persists the consensus params for the next block to disk.
// It should be called from s.Save(), right before the state itself is persisted.
// If the consensus params did not change after processing the latest block,
// only the last height for which they changed is persisted.
func saveConsensusParamsInfo(db dbm.DB, nextHeight, changeHeight int64, params types.ConsensusParams) {
	paramsInfo := &ConsensusParamsInfo{
		LastHeightChanged: changeHeight,
	}
	if changeHeight == nextHeight {
		paramsInfo.ConsensusParams = params
	}
	db.Set(calcConsensusParamsKey(nextHeight), paramsInfo.Bytes())
}
