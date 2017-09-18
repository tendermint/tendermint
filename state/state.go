// Copyright 2014 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package state

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	abci "github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
)

var (
	stateKey         = []byte("stateKey")
	abciResponsesKey = []byte("abciResponsesKey")
)

func calcValidatorsKey(height int) []byte {
	return []byte(cmn.Fmt("validatorsKey:%v", height))
}

//-----------------------------------------------------------------------------

// State represents the latest committed state of the Tendermint consensus,
// including the last committed block and validator set.
// Newly committed blocks are validated and executed against the State.
// NOTE: not goroutine-safe.
type State struct {
	// mtx for writing to db
	mtx sync.Mutex
	db  dbm.DB

	// should not change
	GenesisDoc *types.GenesisDoc
	ChainID    string

	// These fields are updated by SetBlockAndValidators.
	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	// LastValidators is used to validate block.LastCommit.
	LastBlockHeight int
	LastBlockID     types.BlockID
	LastBlockTime   time.Time
	Validators      *types.ValidatorSet
	LastValidators  *types.ValidatorSet

	// AppHash is updated after Commit
	AppHash []byte

	TxIndexer txindex.TxIndexer `json:"-"` // Transaction indexer

	// When a block returns a validator set change via EndBlock,
	// the change only applies to the next block.
	// So, if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	LastHeightValidatorsChanged int

	logger log.Logger
}

// LoadState loads the State from the database.
func LoadState(db dbm.DB) *State {
	return loadState(db, stateKey)
}

func loadState(db dbm.DB, key []byte) *State {
	s := &State{db: db, TxIndexer: &null.TxIndex{}}
	buf := db.Get(key)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&s, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			cmn.Exit(cmn.Fmt("LoadState: Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}

	return s
}

// SetLogger sets the logger on the State.
func (s *State) SetLogger(l log.Logger) {
	s.logger = l
}

// Copy makes a copy of the State for mutating.
func (s *State) Copy() *State {
	return &State{
		db:                          s.db,
		GenesisDoc:                  s.GenesisDoc,
		ChainID:                     s.ChainID,
		LastBlockHeight:             s.LastBlockHeight,
		LastBlockID:                 s.LastBlockID,
		LastBlockTime:               s.LastBlockTime,
		Validators:                  s.Validators.Copy(),
		LastValidators:              s.LastValidators.Copy(),
		AppHash:                     s.AppHash,
		TxIndexer:                   s.TxIndexer, // pointer here, not value
		LastHeightValidatorsChanged: s.LastHeightValidatorsChanged,
		logger: s.logger,
	}
}

// Save persists the State to the database.
func (s *State) Save() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.saveValidatorsInfo()
	s.db.SetSync(stateKey, s.Bytes())
}

// SaveABCIResponses persists the ABCIResponses to the database.
// This is useful in case we crash after app.Commit and before s.Save().
func (s *State) SaveABCIResponses(abciResponses *ABCIResponses) {
	s.db.SetSync(abciResponsesKey, abciResponses.Bytes())
}

// LoadABCIResponses loads the ABCIResponses from the database.
func (s *State) LoadABCIResponses() *ABCIResponses {
	abciResponses := new(ABCIResponses)

	buf := s.db.Get(abciResponsesKey)
	if len(buf) != 0 {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(abciResponses, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			cmn.Exit(cmn.Fmt("LoadABCIResponses: Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}
	return abciResponses
}

// LoadValidators loads the ValidatorSet for a given height.
func (s *State) LoadValidators(height int) (*types.ValidatorSet, error) {
	v := s.loadValidators(height)
	if v == nil {
		return nil, ErrNoValSetForHeight{height}
	}

	if v.ValidatorSet == nil {
		v = s.loadValidators(v.LastHeightChanged)
		if v == nil {
			cmn.PanicSanity(fmt.Sprintf(`Couldn't find validators at 
			height %d as last changed from height %d`, v.LastHeightChanged, height))
		}
	}

	return v.ValidatorSet, nil
}

func (s *State) loadValidators(height int) *ValidatorsInfo {
	buf := s.db.Get(calcValidatorsKey(height))
	if len(buf) == 0 {
		return nil
	}

	v := new(ValidatorsInfo)
	r, n, err := bytes.NewReader(buf), new(int), new(error)
	wire.ReadBinaryPtr(v, r, 0, n, err)
	if *err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.Exit(cmn.Fmt("LoadValidators: Data has been corrupted or its spec has changed: %v\n", *err))
	}
	// TODO: ensure that buf is completely read.
	return v
}

// saveValidatorsInfo persists the validator set for the next block to disk.
// It should be called from s.Save(), right before the state itself is persisted.
// If the validator set did not change after processing the latest block,
// only the last height for which the validators changed is persisted.
func (s *State) saveValidatorsInfo() {
	changeHeight := s.LastHeightValidatorsChanged
	nextHeight := s.LastBlockHeight + 1
	vi := &ValidatorsInfo{
		LastHeightChanged: changeHeight,
	}
	if changeHeight == nextHeight {
		vi.ValidatorSet = s.Validators
	}
	s.db.SetSync(calcValidatorsKey(nextHeight), vi.Bytes())
}

// Equals returns true if the States are identical.
func (s *State) Equals(s2 *State) bool {
	return bytes.Equal(s.Bytes(), s2.Bytes())
}

// Bytes serializes the State using go-wire.
func (s *State) Bytes() []byte {
	return wire.BinaryBytes(s)
}

// SetBlockAndValidators mutates State variables to update block and validators after running EndBlock.
func (s *State) SetBlockAndValidators(header *types.Header, blockPartsHeader types.PartSetHeader, abciResponses *ABCIResponses) {

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	prevValSet := s.Validators.Copy()
	nextValSet := prevValSet.Copy()

	// update the validator set with the latest abciResponses
	if len(abciResponses.EndBlock.Diffs) > 0 {
		err := updateValidators(nextValSet, abciResponses.EndBlock.Diffs)
		if err != nil {
			s.logger.Error("Error changing validator set", "err", err)
			// TODO: err or carry on?
		}
		// change results from this height but only applies to the next height
		s.LastHeightValidatorsChanged = header.Height + 1
	}

	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)

	s.setBlockAndValidators(header.Height,
		types.BlockID{header.Hash(), blockPartsHeader},
		header.Time,
		prevValSet, nextValSet)

}

func (s *State) setBlockAndValidators(
	height int, blockID types.BlockID, blockTime time.Time,
	prevValSet, nextValSet *types.ValidatorSet) {

	s.LastBlockHeight = height
	s.LastBlockID = blockID
	s.LastBlockTime = blockTime
	s.Validators = nextValSet
	s.LastValidators = prevValSet
}

// GetValidators returns the last and current validator sets.
func (s *State) GetValidators() (*types.ValidatorSet, *types.ValidatorSet) {
	return s.LastValidators, s.Validators
}

// GetState loads the most recent state from the database,
// or creates a new one from the given genesisFile and persists the result
// to the database.
func GetState(stateDB dbm.DB, genesisFile string) *State {
	state := LoadState(stateDB)
	if state == nil {
		state = MakeGenesisStateFromFile(stateDB, genesisFile)
		state.Save()
	}

	return state
}

//------------------------------------------------------------------------

// ABCIResponses retains the responses of the various ABCI calls during block processing.
// It is persisted to disk before calling Commit.
type ABCIResponses struct {
	Height int

	DeliverTx []*abci.ResponseDeliverTx
	EndBlock  abci.ResponseEndBlock

	txs types.Txs // reference for indexing results by hash
}

// NewABCIResponses returns a new ABCIResponses
func NewABCIResponses(block *types.Block) *ABCIResponses {
	return &ABCIResponses{
		Height:    block.Height,
		DeliverTx: make([]*abci.ResponseDeliverTx, block.NumTxs),
		txs:       block.Data.Txs,
	}
}

// Bytes serializes the ABCIResponse using go-wire
func (a *ABCIResponses) Bytes() []byte {
	return wire.BinaryBytes(*a)
}

//-----------------------------------------------------------------------------

// ValidatorsInfo represents the latest validator set, or the last time it changed
type ValidatorsInfo struct {
	ValidatorSet      *types.ValidatorSet
	LastHeightChanged int
}

// Bytes serializes the ValidatorsInfo using go-wire
func (vi *ValidatorsInfo) Bytes() []byte {
	return wire.BinaryBytes(*vi)
}

//------------------------------------------------------------------------
// Genesis

// MakeGenesisStateFromFile reads and unmarshals state from the given
// file.
//
// Used during replay and in tests.
func MakeGenesisStateFromFile(db dbm.DB, genDocFile string) *State {
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		cmn.Exit(cmn.Fmt("Couldn't read GenesisDoc file: %v", err))
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading GenesisDoc: %v", err))
	}
	return MakeGenesisState(db, genDoc)
}

// MakeGenesisState creates state from types.GenesisDoc.
//
// Used in tests.
func MakeGenesisState(db dbm.DB, genDoc *types.GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		cmn.Exit(cmn.Fmt("The genesis file has no validators"))
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	// Make validators slice
	validators := make([]*types.Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		pubKey := val.PubKey
		address := pubKey.Address()

		// Make validator
		validators[i] = &types.Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Amount,
		}
	}

	return &State{
		db:                          db,
		GenesisDoc:                  genDoc,
		ChainID:                     genDoc.ChainID,
		LastBlockHeight:             0,
		LastBlockID:                 types.BlockID{},
		LastBlockTime:               genDoc.GenesisTime,
		Validators:                  types.NewValidatorSet(validators),
		LastValidators:              types.NewValidatorSet(nil),
		AppHash:                     genDoc.AppHash,
		TxIndexer:                   &null.TxIndex{}, // we do not need indexer during replay and in tests
		LastHeightValidatorsChanged: 1,
	}
}
