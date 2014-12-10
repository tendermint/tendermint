package state

import (
	"encoding/json"
	"io/ioutil"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

type GenesisDoc struct {
	GenesisTime time.Time
	Accounts    []*Account
	Validators  []*Validator
}

func GenesisDocFromJSON(jsonBlob []byte) (genState *GenesisDoc) {
	err := json.Unmarshal(jsonBlob, &genState)
	if err != nil {
		Panicf("Couldn't read GenesisDoc: %v", err)
	}
	return
}

func GenesisStateFromFile(db db_.DB, genDocFile string) *State {
	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		Panicf("Couldn't read GenesisDoc file: %v", err)
	}
	genDoc := GenesisDocFromJSON(jsonBlob)
	return GenesisState(db, genDoc)
}

func GenesisState(db db_.DB, genDoc *GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		panic("Must have some validators")
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	// Make accounts state tree
	accounts := merkle.NewIAVLTree(BasicCodec, AccountCodec, defaultAccountsCacheCapacity, db)
	for _, acc := range genDoc.Accounts {
		accounts.Set(acc.Address, acc)
	}

	return &State{
		DB:                  db,
		LastBlockHeight:     0,
		LastBlockHash:       nil,
		LastBlockParts:      PartSetHeader{},
		LastBlockTime:       genDoc.GenesisTime,
		BondedValidators:    NewValidatorSet(genDoc.Validators),
		UnbondingValidators: NewValidatorSet(nil),
		accounts:            accounts,
	}
}
