package types

import (
	"time"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/wire"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

type BasicAccount struct {
	Address []byte `json:"address"`
	Amount  int64  `json:"amount"`
}

type GenesisAccount struct {
	Address     []byte                     `json:"address"`
	Amount      int64                      `json:"amount"`
	Name        string                     `json:"name"`
	Permissions *ptypes.AccountPermissions `json:"permissions"`
}

type GenesisValidator struct {
	PubKey   acm.PubKeyEd25519 `json:"pub_key"`
	Amount   int64             `json:"amount"`
	Name     string            `json:"name"`
	UnbondTo []BasicAccount    `json:"unbond_to"`
}

type GenesisParams struct {
	GlobalPermissions *ptypes.AccountPermissions `json:"global_permissions"`
}

type GenesisDoc struct {
	GenesisTime time.Time          `json:"genesis_time"`
	ChainID     string             `json:"chain_id"`
	Params      *GenesisParams     `json:"params"`
	Accounts    []GenesisAccount   `json:"accounts"`
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
