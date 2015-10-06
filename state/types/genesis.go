package types

import (
	"sort"
	"time"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/types"
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

//------------------------------------------------------------
// Make random genesis state

func RandAccount(randBalance bool, minBalance int64) (*acm.Account, *acm.PrivAccount) {
	privAccount := acm.GenPrivAccount()
	perms := ptypes.DefaultAccountPermissions
	acc := &acm.Account{
		Address:     privAccount.PubKey.Address(),
		PubKey:      privAccount.PubKey,
		Sequence:    RandInt(),
		Balance:     minBalance,
		Permissions: perms,
	}
	if randBalance {
		acc.Balance += int64(RandUint32())
	}
	return acc, privAccount
}

func RandGenesisDoc(numAccounts int, randBalance bool, minBalance int64, numValidators int, randBonded bool, minBonded int64) (*GenesisDoc, []*acm.PrivAccount, []*types.PrivValidator) {
	accounts := make([]GenesisAccount, numAccounts)
	privAccounts := make([]*acm.PrivAccount, numAccounts)
	defaultPerms := ptypes.DefaultAccountPermissions
	for i := 0; i < numAccounts; i++ {
		account, privAccount := RandAccount(randBalance, minBalance)
		accounts[i] = GenesisAccount{
			Address:     account.Address,
			Amount:      account.Balance,
			Permissions: &defaultPerms, // This will get copied into each state.Account.
		}
		privAccounts[i] = privAccount
	}
	validators := make([]GenesisValidator, numValidators)
	privValidators := make([]*types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		valInfo, _, privVal := types.RandValidator(randBonded, minBonded)
		validators[i] = GenesisValidator{
			PubKey: valInfo.PubKey,
			Amount: valInfo.FirstBondAmount,
			UnbondTo: []BasicAccount{
				{
					Address: valInfo.PubKey.Address(),
					Amount:  valInfo.FirstBondAmount,
				},
			},
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))
	return &GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     "tendermint_test",
		Accounts:    accounts,
		Validators:  validators,
	}, privAccounts, privValidators

}
