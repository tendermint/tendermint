package state

import (
	"bytes"
	"io/ioutil"
	"time"

	. "github.com/tendermint/go-common"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
)

var (
	stateKey = []byte("stateKey")
)

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB              dbm.DB
	ChainID         string
	LastBlockHeight int
	LastBlockHash   []byte
	LastBlockParts  types.PartSetHeader
	LastBlockTime   time.Time
	Validators      *types.ValidatorSet
	LastValidators  *types.ValidatorSet

	evc events.Fireable // typically an events.EventCache
}

func LoadState(db dbm.DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int64), new(error)
		s.ChainID = wire.ReadString(r, n, err)
		s.LastBlockHeight = wire.ReadVarint(r, n, err)
		s.LastBlockHash = wire.ReadByteSlice(r, n, err)
		s.LastBlockParts = wire.ReadBinary(types.PartSetHeader{}, r, n, err).(types.PartSetHeader)
		s.LastBlockTime = wire.ReadTime(r, n, err)
		s.Validators = wire.ReadBinary(&types.ValidatorSet{}, r, n, err).(*types.ValidatorSet)
		s.LastValidators = wire.ReadBinary(&types.ValidatorSet{}, r, n, err).(*types.ValidatorSet)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			Exit(Fmt("Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

func (s *State) Save() {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	wire.WriteString(s.ChainID, buf, n, err)
	wire.WriteVarint(s.LastBlockHeight, buf, n, err)
	wire.WriteByteSlice(s.LastBlockHash, buf, n, err)
	wire.WriteBinary(s.LastBlockParts, buf, n, err)
	wire.WriteTime(s.LastBlockTime, buf, n, err)
	wire.WriteBinary(s.Validators, buf, n, err)
	wire.WriteBinary(s.LastValidators, buf, n, err)
	if *err != nil {
		PanicCrisis(*err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

// CONTRACT:
// Copy() is a cheap way to take a snapshot,
// as if State were copied by value.
func (s *State) Copy() *State {
	return &State{
		DB:              s.DB,
		ChainID:         s.ChainID,
		LastBlockHeight: s.LastBlockHeight,
		LastBlockHash:   s.LastBlockHash,
		LastBlockParts:  s.LastBlockParts,
		LastBlockTime:   s.LastBlockTime,
		Validators:      s.Validators.Copy(),     // TODO remove need for Copy() here.
		LastValidators:  s.LastValidators.Copy(), // That is, make updates to the validator set
		evc:             nil,
	}
}

// Returns a hash that represents the state data, excluding Last*
func (s *State) Hash() []byte {
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"Validators": s.Validators,
	})
}

// Mutates the block in place and updates it with new state hash.
func (s *State) ComputeBlockStateHash(block *types.Block) error {
	sCopy := s.Copy()
	// sCopy has no event cache in it, so this won't fire events
	err := execBlock(sCopy, block, types.PartSetHeader{})
	if err != nil {
		return err
	}
	// Set block.StateHash
	block.StateHash = sCopy.Hash()
	return nil
}

func (s *State) SetDB(db dbm.DB) {
	s.DB = db
}

// Implements events.Eventable. Typically uses events.EventCache
func (s *State) SetFireable(evc events.Fireable) {
	s.evc = evc
}

//-----------------------------------------------------------------------------
// Genesis

func MakeGenesisStateFromFile(db dbm.DB, genDocFile string) (*types.GenesisDoc, *State) {
	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
	}
	genDoc := types.GenesisDocFromJSON(jsonBlob)
	return genDoc, MakeGenesisState(db, genDoc)
}

func MakeGenesisState(db dbm.DB, genDoc *types.GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		Exit(Fmt("The genesis file has no validators"))
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	// XXX Speak to application, ensure genesis state.

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
		DB:              db,
		ChainID:         genDoc.ChainID,
		LastBlockHeight: 0,
		LastBlockHash:   nil,
		LastBlockParts:  types.PartSetHeader{},
		LastBlockTime:   genDoc.GenesisTime,
		Validators:      types.NewValidatorSet(validators),
		LastValidators:  types.NewValidatorSet(nil),
	}
}

func RandGenesisState(numValidators int, randPower bool, minPower int64) (*State, []*types.PrivValidator) {
	db := dbm.NewMemDB()
	genDoc, privValidators := types.RandGenesisDoc(numValidators, randPower, minPower)
	s0 := MakeGenesisState(db, genDoc)
	s0.Save()
	return s0, privValidators
}
