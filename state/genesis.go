package state

import (
	"encoding/json"
	"io/ioutil"
	"time"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

type GenesisDoc struct {
	GenesisTime time.Time
	Accounts    []*Account
	Validators  []*ValidatorInfo
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

	// Make validatorInfos state tree
	validatorInfos := merkle.NewIAVLTree(BasicCodec, ValidatorInfoCodec, 0, db)
	for _, valInfo := range genDoc.Validators {
		validatorInfos.Set(valInfo.Address, valInfo)
	}

	// Make validators
	validators := make([]*Validator, len(genDoc.Validators))
	for i, valInfo := range genDoc.Validators {
		validators[i] = &Validator{
			Address:     valInfo.Address,
			PubKey:      valInfo.PubKey,
			VotingPower: valInfo.FirstBondAmount,
		}
	}

	return &State{
		DB:                  db,
		LastBlockHeight:     0,
		LastBlockHash:       nil,
		LastBlockParts:      PartSetHeader{},
		LastBlockTime:       genDoc.GenesisTime,
		BondedValidators:    NewValidatorSet(validators),
		UnbondingValidators: NewValidatorSet(nil),
		accounts:            accounts,
		validatorInfos:      validatorInfos,
	}
}
