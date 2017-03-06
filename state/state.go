package state

import (
	"bytes"
	"io/ioutil"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

var (
	stateKey = []byte("stateKey")
)

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	// mtx for writing to db
	mtx sync.Mutex
	db  dbm.DB

	// should not change
	GenesisDoc *types.GenesisDoc
	ChainID    string

	// updated at end of ExecBlock
	LastBlockHeight int // Genesis state has this set to 0.  So, Block(H=0) does not exist.
	LastBlockID     types.BlockID
	LastBlockTime   time.Time
	Validators      *types.ValidatorSet
	LastValidators  *types.ValidatorSet // block.LastCommit validated against this

	// AppHash is updated after Commit
	AppHash []byte
}

func LoadState(db dbm.DB) *State {
	return loadState(db, stateKey)
}

func loadState(db dbm.DB, key []byte) *State {
	s := &State{db: db}
	buf := db.Get(key)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&s, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			Exit(Fmt("Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

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
	}
}

func (s *State) Save() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.db.SetSync(stateKey, s.Bytes())
}

func (s *State) Equals(s2 *State) bool {
	return bytes.Equal(s.Bytes(), s2.Bytes())
}

func (s *State) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(s, buf, n, err)
	if *err != nil {
		PanicCrisis(*err)
	}
	return buf.Bytes()
}

// Mutate state variables to match block and validators
// after running EndBlock
func (s *State) SetBlockAndValidators(header *types.Header, blockPartsHeader types.PartSetHeader, prevValSet, nextValSet *types.ValidatorSet) {
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

func (s *State) GetValidators() (*types.ValidatorSet, *types.ValidatorSet) {
	return s.LastValidators, s.Validators
}

// Load the most recent state from "state" db,
// or create a new one (and save) from genesis.
func GetState(config cfg.Config, stateDB dbm.DB) *State {
	state := LoadState(stateDB)
	if state == nil {
		state = MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))
		state.Save()
	}
	return state
}

//-----------------------------------------------------------------------------
// Genesis

func MakeGenesisStateFromFile(db dbm.DB, genDocFile string) *State {
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		Exit(Fmt("Error reading GenesisDoc: %v", err))
	}
	return MakeGenesisState(db, genDoc)
}

func MakeGenesisState(db dbm.DB, genDoc *types.GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		Exit(Fmt("The genesis file has no validators"))
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
	}
}
