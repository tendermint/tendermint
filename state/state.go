package state

import (
	"bytes"
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

	// updated at end of SetBlockAndValidators
	LastBlockHeight int // Genesis state has this set to 0.  So, Block(H=0) does not exist.
	LastBlockID     types.BlockID
	LastBlockTime   time.Time
	Validators      *types.ValidatorSet
	LastValidators  *types.ValidatorSet // block.LastCommit validated against this

	// AppHash is updated after Commit
	AppHash []byte

	TxIndexer txindex.TxIndexer `json:"-"` // Transaction indexer.

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
		db:              s.db,
		GenesisDoc:      s.GenesisDoc,
		ChainID:         s.ChainID,
		LastBlockHeight: s.LastBlockHeight,
		LastBlockID:     s.LastBlockID,
		LastBlockTime:   s.LastBlockTime,
		Validators:      s.Validators.Copy(),
		LastValidators:  s.LastValidators.Copy(),
		AppHash:         s.AppHash,
		TxIndexer:       s.TxIndexer, // pointer here, not value
		logger:          s.logger,
	}
}

// Save persists the State to the database.
func (s *State) Save() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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

	err := updateValidators(nextValSet, abciResponses.EndBlock.Diffs)
	if err != nil {
		s.logger.Error("Error changing validator set", "err", err)
		// TODO: err or carry on?
	}

	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)

	s.setBlockAndValidators(header.Height,
		types.BlockID{header.Hash(), blockPartsHeader}, header.Time,
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

//--------------------------------------------------

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
// Genesis

// MakeGenesisStateFromFile reads and unmarshals state from the given file.
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
		db:              db,
		GenesisDoc:      genDoc,
		ChainID:         genDoc.ChainID,
		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   genDoc.GenesisTime,
		Validators:      types.NewValidatorSet(validators),
		LastValidators:  types.NewValidatorSet(nil),
		AppHash:         genDoc.AppHash,
		TxIndexer:       &null.TxIndex{}, // we do not need indexer during replay and in tests
	}
}
