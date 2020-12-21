package state

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	// https://github.com/tendermint/tendermint/pull/3438
	// 100000 results in ~ 100ms to get 100 validators (see BenchmarkLoadValidators)
	valSetCheckpointInterval = 100000
)

//------------------------------------------------------------------------

const (
	// prefixes are unique across all tm db's
	prefixValidators      = byte(0x05)
	prefixConsensusParams = byte(0x06)
	prefixABCIResponses   = byte(0x07)
)

func encodeKey(buf []byte, height int64) []byte {
	res, _ := orderedcode.Append(buf, height)
	return res
}

func decodeKey(key []byte) int64 {
	var height int64
	_, err := orderedcode.Parse(string(key[1:]), &height)
	if err != nil {
		panic(err)
	}
	return height
}

func validatorsKey(height int64) []byte {
	return encodeKey([]byte{prefixValidators}, height)
}

func consensusParamsKey(height int64) []byte {
	return encodeKey([]byte{prefixConsensusParams}, height)
}

func abciResponsesKey(height int64) []byte {
	return encodeKey([]byte{prefixABCIResponses}, height)
}

//----------------------

//go:generate mockery --case underscore --name Store

// Store defines the state store interface
//
// It is used to retrieve current state and save and load ABCI responses,
// validators and consensus parameters
type Store interface {
	// LoadFromDBOrGenesisFile loads the most recent state.
	// If the chain is new it will use the genesis file from the provided genesis file path as the current state.
	LoadFromDBOrGenesisFile(string) (State, error)
	// LoadFromDBOrGenesisDoc loads the most recent state.
	// If the chain is new it will use the genesis doc as the current state.
	LoadFromDBOrGenesisDoc(*types.GenesisDoc) (State, error)
	// Load loads the current state of the blockchain
	Load() (State, error)
	// LoadValidators loads the validator set at a given height
	LoadValidators(int64) (*types.ValidatorSet, error)
	// LoadABCIResponses loads the abciResponse for a given height
	LoadABCIResponses(int64) (*tmstate.ABCIResponses, error)
	// LoadConsensusParams loads the consensus params for a given height
	LoadConsensusParams(int64) (tmproto.ConsensusParams, error)
	// Save overwrites the previous state with the updated one
	Save(State) error
	// SaveABCIResponses saves ABCIResponses for a given height
	SaveABCIResponses(int64, *tmstate.ABCIResponses) error
	// Bootstrap is used for bootstrapping state when not starting from a initial height.
	Bootstrap(State) error
	// PruneStates takes the height from which to finish pruning at (exclusive)
	PruneStates(int64) error
}

// dbStore wraps a db (github.com/tendermint/tm-db)
type dbStore struct {
	db dbm.DB
}

var _ Store = (*dbStore)(nil)

// NewStore creates the dbStore of the state pkg.
func NewStore(db dbm.DB) Store {
	return dbStore{db}
}

// LoadStateFromDBOrGenesisFile loads the most recent state from the database,
// or creates a new one from the given genesisFilePath.
func (store dbStore) LoadFromDBOrGenesisFile(genesisFilePath string) (State, error) {
	state, err := store.Load()
	if err != nil {
		return State{}, err
	}
	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisStateFromFile(genesisFilePath)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

// LoadStateFromDBOrGenesisDoc loads the most recent state from the database,
// or creates a new one from the given genesisDoc.
func (store dbStore) LoadFromDBOrGenesisDoc(genesisDoc *types.GenesisDoc) (State, error) {
	state, err := store.Load()
	if err != nil {
		return State{}, err
	}

	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisState(genesisDoc)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

// LoadState loads the State from the database.
func (store dbStore) Load() (State, error) {
	return store.loadState(stateKey)
}

func (store dbStore) loadState(key []byte) (state State, err error) {
	buf, err := store.db.Get(key)
	if err != nil {
		return state, err
	}
	if len(buf) == 0 {
		return state, nil
	}

	sp := new(tmstate.State)

	err = proto.Unmarshal(buf, sp)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		tmos.Exit(fmt.Sprintf(`LoadState: Data has been corrupted or its spec has changed:
		%v\n`, err))
	}

	sm, err := StateFromProto(sp)
	if err != nil {
		return state, err
	}

	return *sm, nil
}

// Save persists the State, the ValidatorsInfo, and the ConsensusParamsInfo to the database.
// This flushes the writes (e.g. calls SetSync).
func (store dbStore) Save(state State) error {
	return store.save(state, stateKey)
}

func (store dbStore) save(state State, key []byte) error {
	nextHeight := state.LastBlockHeight + 1
	// If first block, save validators for the block.
	if nextHeight == 1 {
		nextHeight = state.InitialHeight
		// This extra logic due to Tendermint validator set changes being delayed 1 block.
		// It may get overwritten due to InitChain validator updates.
		if err := store.saveValidatorsInfo(nextHeight, nextHeight, state.Validators); err != nil {
			return err
		}
	}
	// Save next validators.
	if err := store.saveValidatorsInfo(nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators); err != nil {
		return err
	}

	// Save next consensus params.
	if err := store.saveConsensusParamsInfo(nextHeight,
		state.LastHeightConsensusParamsChanged, state.ConsensusParams); err != nil {
		return err
	}
	err := store.db.SetSync(key, state.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// BootstrapState saves a new state, used e.g. by state sync when starting from non-zero height.
func (store dbStore) Bootstrap(state State) error {
	height := state.LastBlockHeight + 1
	if height == 1 {
		height = state.InitialHeight
	}

	if height > 1 && !state.LastValidators.IsNilOrEmpty() {
		if err := store.saveValidatorsInfo(height-1, height-1, state.LastValidators); err != nil {
			return err
		}
	}

	if err := store.saveValidatorsInfo(height, height, state.Validators); err != nil {
		return err
	}

	if err := store.saveValidatorsInfo(height+1, height+1, state.NextValidators); err != nil {
		return err
	}

	if err := store.saveConsensusParamsInfo(height, height, state.ConsensusParams); err != nil {
		return err
	}

	return store.db.SetSync(stateKey, state.Bytes())
}

// PruneStates deletes states up to the height specified (exclusive). It is not
// guaranteed to delete all states, since the last checkpointed state and states being pointed to by
// e.g. `LastHeightChanged` must remain. The state at retain height must also exist.
// Pruning is done in ascending order.
func (store dbStore) PruneStates(retainHeight int64) error {
	if retainHeight <= 0 {
		return fmt.Errorf("height %v must be greater than 0", retainHeight)
	}

	if err := store.pruneValidatorSets(retainHeight); err != nil {
		return err
	}

	if err := store.pruneConsensusParams(retainHeight); err != nil {
		return err
	}

	if err := store.pruneABCIResponses(retainHeight); err != nil {
		return err
	}

	return nil
}

func (store dbStore) pruneValidatorSets(height int64) error {
	valInfo, err := loadValidatorsInfo(store.db, height)
	if err != nil {
		return fmt.Errorf("validators at height %v not found: %w", height, err)
	}

	var (
		lastRecordedValSetHeight int64
		lastRecordedValSet       *tmstate.ValidatorsInfo
	)

	// We will prune up to the validator set at the given "height". As we don't save validator sets every
	// height but only when they change or at a check point, it is likely that the validator set at the height
	// we prune to is empty and thus dependent on the validator set saved at a previous height. We must find
	// that validator set and make sure it is not pruned by saving after we have finished pruning.
	if valInfo.ValidatorSet == nil {
		lastRecordedValSetHeight = lastStoredHeightFor(height, valInfo.LastHeightChanged)
		lastRecordedValSet, err = loadValidatorsInfo(store.db, lastRecordedValSetHeight)
		if err != nil || lastRecordedValSet.ValidatorSet == nil {
			return fmt.Errorf("couldn't find validators at height %d (height %d was originally requested): %w",
				lastStoredHeightFor(height, valInfo.LastHeightChanged),
				height,
				err,
			)
		}
	}

	// batch delete all the validators sets up to height
	if err := store.batchDelete(validatorsKey, height); err != nil {
		return err
	}

	// now we recover the last recorded validator set if it had been set earlier
	if lastRecordedValSet != nil {
		bz, err := lastRecordedValSet.Marshal()
		if err != nil {
			return err
		}
		if err := store.db.Set(validatorsKey(lastRecordedValSetHeight), bz); err != nil {
			return err
		}
	}

	return nil
}

// pruneConsensusParams calls an iterator from base height to retain height batch deleting
// all consensus params in between. If the consensus params at the new base height is dependent
// on a prior height then this will keep that lower height to.
func (store dbStore) pruneConsensusParams(height int64) error {
	paramsInfo, err := store.loadConsensusParamsInfo(height)
	if err != nil {
		return fmt.Errorf("consensus params at height %v not found: %w", height, err)
	}

	// As we don't save the consensus params at every height, only when there is a consensus params change,
	// we must not prune (or save) the last consensus params that the consensus params info at height
	// is dependent on.
	var lastRecordedConsensusParams *tmstate.ConsensusParamsInfo
	if paramsInfo.ConsensusParams.Equal(&tmproto.ConsensusParams{}) {
		lastRecordedConsensusParams, err = store.loadConsensusParamsInfo(paramsInfo.LastHeightChanged)
		if err != nil || lastRecordedConsensusParams.ConsensusParams.Equal(&tmproto.ConsensusParams{}) {
			return fmt.Errorf(
				"couldn't find consensus params at height %d as last changed from height %d: %w",
				paramsInfo.LastHeightChanged,
				height,
				err,
			)
		}
	}

	// batch delete all the consensus params up to height
	if err := store.batchDelete(consensusParamsKey, height); err != nil {
		return err
	}

	// check if we had a dependent consensus params info of an earlier height that we need to save.
	// If so then we restore it now
	if !lastRecordedConsensusParams.ConsensusParams.Equal(&tmproto.ConsensusParams{}) {
		bz, err := lastRecordedConsensusParams.Marshal()
		if err != nil {
			return err
		}

		if err := store.db.Set(consensusParamsKey(paramsInfo.LastHeightChanged), bz); err != nil {
			return err
		}
	}

	return nil
}

// pruneABCIResponses calls an iterator from base height to retain height batch deleting
// all abci responses in between
func (store dbStore) pruneABCIResponses(height int64) error {
	return store.batchDelete(abciResponsesKey, height)
}

// batchDelete is a generic function for deleting a range of values based on the lowest
// height up to but excluding retainHeight
func (store dbStore) batchDelete(key func(int64) []byte, retainHeight int64) error {
	iter, err := store.db.Iterator(
		key(1),
		key(retainHeight),
	)
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	batch := store.db.NewBatch()
	defer batch.Close()

	pruned := 0
	for iter.Valid() {
		if err := batch.Delete(iter.Key()); err != nil {
			return fmt.Errorf("pruning error at height %d: %w", decodeKey(iter.Key()), err)
		}

		pruned++
		// avoid batches growing too large by flushing to database regularly
		if pruned%1000 == 0 {
			if err := iter.Error(); err != nil {
				return err
			}
			if err := iter.Close(); err != nil {
				return err
			}

			err := batch.Write()
			if err != nil {
				return fmt.Errorf("pruning error at height %d: %w", decodeKey(iter.Key()), err)
			}
			if err := batch.Close(); err != nil {
				return err
			}

			iter, err = store.db.Iterator(
				key(1),
				key(retainHeight),
			)
			if err != nil {
				panic(err)
			}
			defer iter.Close()

			batch = store.db.NewBatch()
			defer batch.Close()
		} else {
			iter.Next()
		}
	}
	if err := iter.Error(); err != nil {
		return err
	}

	err = batch.WriteSync()
	if err != nil {
		return fmt.Errorf("pruning error at height %d: %w", decodeKey(iter.Key()), err)
	}

	return nil
}

//------------------------------------------------------------------------

// ABCIResponsesResultsHash returns the root hash of a Merkle tree of
// ResponseDeliverTx responses (see ABCIResults.Hash)
//
// See merkle.SimpleHashFromByteSlices
func ABCIResponsesResultsHash(ar *tmstate.ABCIResponses) []byte {
	return types.NewResults(ar.DeliverTxs).Hash()
}

// LoadABCIResponses loads the ABCIResponses for the given height from the
// database. If not found, ErrNoABCIResponsesForHeight is returned.
//
// This is useful for recovering from crashes where we called app.Commit and
// before we called s.Save(). It can also be used to produce Merkle proofs of
// the result of txs.
func (store dbStore) LoadABCIResponses(height int64) (*tmstate.ABCIResponses, error) {
	buf, err := store.db.Get(abciResponsesKey(height))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {

		return nil, ErrNoABCIResponsesForHeight{height}
	}

	abciResponses := new(tmstate.ABCIResponses)
	err = abciResponses.Unmarshal(buf)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		tmos.Exit(fmt.Sprintf(`LoadABCIResponses: Data has been corrupted or its spec has
                changed: %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return abciResponses, nil
}

// SaveABCIResponses persists the ABCIResponses to the database.
// This is useful in case we crash after app.Commit and before s.Save().
// Responses are indexed by height so they can also be loaded later to produce
// Merkle proofs.
//
// Exposed for testing.
func (store dbStore) SaveABCIResponses(height int64, abciResponses *tmstate.ABCIResponses) error {
	var dtxs []*abci.ResponseDeliverTx
	// strip nil values,
	for _, tx := range abciResponses.DeliverTxs {
		if tx != nil {
			dtxs = append(dtxs, tx)
		}
	}
	abciResponses.DeliverTxs = dtxs

	bz, err := abciResponses.Marshal()
	if err != nil {
		return err
	}

	err = store.db.SetSync(abciResponsesKey(height), bz)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func (store dbStore) LoadValidators(height int64) (*types.ValidatorSet, error) {

	valInfo, err := loadValidatorsInfo(store.db, height)
	if err != nil {
		return nil, ErrNoValSetForHeight{height}
	}
	if valInfo.ValidatorSet == nil {
		lastStoredHeight := lastStoredHeightFor(height, valInfo.LastHeightChanged)
		valInfo2, err := loadValidatorsInfo(store.db, lastStoredHeight)
		if err != nil || valInfo2.ValidatorSet == nil {
			return nil,
				fmt.Errorf("couldn't find validators at height %d (height %d was originally requested): %w",
					lastStoredHeight,
					height,
					err,
				)
		}

		vs, err := types.ValidatorSetFromProto(valInfo2.ValidatorSet)
		if err != nil {
			return nil, err
		}

		vs.IncrementProposerPriority(tmmath.SafeConvertInt32(height - lastStoredHeight)) // mutate
		vi2, err := vs.ToProto()
		if err != nil {
			return nil, err
		}

		valInfo2.ValidatorSet = vi2
		valInfo = valInfo2
	}

	vip, err := types.ValidatorSetFromProto(valInfo.ValidatorSet)
	if err != nil {
		return nil, err
	}

	return vip, nil
}

func lastStoredHeightFor(height, lastHeightChanged int64) int64 {
	checkpointHeight := height - height%valSetCheckpointInterval
	return tmmath.MaxInt64(checkpointHeight, lastHeightChanged)
}

// CONTRACT: Returned ValidatorsInfo can be mutated.
func loadValidatorsInfo(db dbm.DB, height int64) (*tmstate.ValidatorsInfo, error) {
	buf, err := db.Get(validatorsKey(height))
	if err != nil {
		return nil, err
	}

	if len(buf) == 0 {
		return nil, errors.New("value retrieved from db is empty")
	}

	v := new(tmstate.ValidatorsInfo)
	err = v.Unmarshal(buf)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		tmos.Exit(fmt.Sprintf(`LoadValidators: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return v, nil
}

// saveValidatorsInfo persists the validator set.
//
// We expect validator sets to change irregularly
//
// `height` is the effective height for which the validator is responsible for
// signing. It should be called from s.Save(), right before the state itself is
// persisted.
func (store dbStore) saveValidatorsInfo(height, lastHeightChanged int64, valSet *types.ValidatorSet) error {
	if lastHeightChanged > height {
		return errors.New("lastHeightChanged cannot be greater than ValidatorsInfo height")
	}
	valInfo := &tmstate.ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}
	// Only persist validator set if it was updated or checkpoint height (see
	// valSetCheckpointInterval) is reached.
	if height == lastHeightChanged || height%valSetCheckpointInterval == 0 {
		pv, err := valSet.ToProto()
		if err != nil {
			return err
		}
		valInfo.ValidatorSet = pv
	}

	bz, err := valInfo.Marshal()
	if err != nil {
		return err
	}

	err = store.db.Set(validatorsKey(height), bz)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------

// ConsensusParamsInfo represents the latest consensus params, or the last height it changed

// LoadConsensusParams loads the ConsensusParams for a given height.
func (store dbStore) LoadConsensusParams(height int64) (tmproto.ConsensusParams, error) {
	empty := tmproto.ConsensusParams{}

	paramsInfo, err := store.loadConsensusParamsInfo(height)
	if err != nil {
		return empty, fmt.Errorf("could not find consensus params for height #%d: %w", height, err)
	}

	if paramsInfo.ConsensusParams.Equal(&empty) {
		paramsInfo2, err := store.loadConsensusParamsInfo(paramsInfo.LastHeightChanged)
		if err != nil {
			return empty, fmt.Errorf(
				"couldn't find consensus params at height %d as last changed from height %d: %w",
				paramsInfo.LastHeightChanged,
				height,
				err,
			)
		}

		paramsInfo = paramsInfo2
	}

	return paramsInfo.ConsensusParams, nil
}

func (store dbStore) loadConsensusParamsInfo(height int64) (*tmstate.ConsensusParamsInfo, error) {
	buf, err := store.db.Get(consensusParamsKey(height))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, errors.New("value retrieved from db is empty")
	}

	paramsInfo := new(tmstate.ConsensusParamsInfo)
	if err = paramsInfo.Unmarshal(buf); err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		tmos.Exit(fmt.Sprintf(`LoadConsensusParams: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return paramsInfo, nil
}

// saveConsensusParamsInfo persists the consensus params for the next block to disk.
// It should be called from s.Save(), right before the state itself is persisted.
// If the consensus params did not change after processing the latest block,
// only the last height for which they changed is persisted.
func (store dbStore) saveConsensusParamsInfo(nextHeight, changeHeight int64, params tmproto.ConsensusParams) error {
	paramsInfo := &tmstate.ConsensusParamsInfo{
		LastHeightChanged: changeHeight,
	}

	if changeHeight == nextHeight {
		paramsInfo.ConsensusParams = params
	}
	bz, err := paramsInfo.Marshal()
	if err != nil {
		return err
	}

	err = store.db.Set(consensusParamsKey(nextHeight), bz)
	if err != nil {
		return err
	}

	return nil
}
