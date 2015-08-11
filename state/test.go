package state

import (
	"sort"
	"time"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	ptypes "github.com/tendermint/tendermint/permission/types"
	. "github.com/tendermint/tendermint/state/types"
	"github.com/tendermint/tendermint/types"
)

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

func RandGenesisState(numAccounts int, randBalance bool, minBalance int64, numValidators int, randBonded bool, minBonded int64) (*State, []*acm.PrivAccount, []*types.PrivValidator) {
	db := dbm.NewMemDB()
	genDoc, privAccounts, privValidators := RandGenesisDoc(numAccounts, randBalance, minBalance, numValidators, randBonded, minBonded)
	s0 := MakeGenesisState(db, genDoc)
	s0.Save()
	return s0, privAccounts, privValidators
}
