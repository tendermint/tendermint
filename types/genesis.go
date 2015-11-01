package types

import (
	"sort"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

type GenesisValidator struct {
	PubKey crypto.PubKeyEd25519 `json:"pub_key"`
	Amount int64                `json:"amount"`
	Name   string               `json:"name"`
}

type GenesisDoc struct {
	GenesisTime time.Time          `json:"genesis_time"`
	ChainID     string             `json:"chain_id"`
	Validators  []GenesisValidator `json:"validators"`
}

//------------------------------------------------------------
// Make genesis state from file

func GenesisDocFromJSON(jsonBlob []byte) (genState *GenesisDoc) {
	var err error
	wire.ReadJSONPtr(&genState, jsonBlob, &err)
	if err != nil {
		Exit(Fmt("Couldn't read GenesisDoc: %v", err))
	}
	return
}

//------------------------------------------------------------
// Make random genesis state

func RandGenesisDoc(numValidators int, randPower bool, minPower int64) (*GenesisDoc, []*PrivValidator) {
	validators := make([]GenesisValidator, numValidators)
	privValidators := make([]*PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := RandValidator(randPower, minPower)
		validators[i] = GenesisValidator{
			PubKey: val.PubKey,
			Amount: val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return &GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     "tendermint_test",
		Validators:  validators,
	}, privValidators

}
