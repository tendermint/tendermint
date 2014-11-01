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
	GenesisTime    time.Time
	AccountDetails []*AccountDetail
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
	return GenesisStateFromDoc(db, genDoc)
}

func GenesisStateFromDoc(db db_.DB, genDoc *GenesisDoc) *State {
	return GenesisState(db, genDoc.GenesisTime, genDoc.AccountDetails)
}

func GenesisState(db db_.DB, genesisTime time.Time, accDets []*AccountDetail) *State {

	if genesisTime.IsZero() {
		genesisTime = time.Now()
	}

	// TODO: Use "uint64Codec" instead of BasicCodec
	accountDetails := merkle.NewIAVLTree(BasicCodec, AccountDetailCodec, defaultAccountDetailsCacheCapacity, db)
	validators := []*Validator{}

	for _, accDet := range accDets {
		accountDetails.Set(accDet.Id, accDet)
		if accDet.Status == AccountStatusBonded {
			validators = append(validators, &Validator{
				Account:     accDet.Account,
				BondHeight:  0,
				VotingPower: accDet.Balance,
				Accum:       0,
			})
		}
	}

	if len(validators) == 0 {
		panic("Must have some validators")
	}

	return &State{
		DB:                  db,
		LastBlockHeight:     0,
		LastBlockHash:       nil,
		LastBlockParts:      PartSetHeader{},
		LastBlockTime:       genesisTime,
		BondedValidators:    NewValidatorSet(validators),
		UnbondingValidators: NewValidatorSet(nil),
		accountDetails:      accountDetails,
	}
}
