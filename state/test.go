package state

import (
	"bytes"
	"sort"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/types"

	"io/ioutil"
	"os"
	"time"
)

func Tempfile(prefix string) (*os.File, string) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		panic(err)
	}
	return file, file.Name()
}

func RandAccount(randBalance bool, minBalance int64) (*account.Account, *account.PrivAccount) {
	privAccount := account.GenPrivAccount()
	perms := ptypes.DefaultAccountPermissions
	acc := &account.Account{
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

func RandValidator(randBonded bool, minBonded int64) (*ValidatorInfo, *Validator, *PrivValidator) {
	privVal := GenPrivValidator()
	_, tempFilePath := Tempfile("priv_validator_")
	privVal.SetFile(tempFilePath)
	bonded := minBonded
	if randBonded {
		bonded += int64(RandUint32())
	}
	valInfo := &ValidatorInfo{
		Address: privVal.Address,
		PubKey:  privVal.PubKey,
		UnbondTo: []*types.TxOutput{&types.TxOutput{
			Amount:  bonded,
			Address: privVal.Address,
		}},
		FirstBondHeight: 0,
		FirstBondAmount: bonded,
	}
	val := &Validator{
		Address:          valInfo.Address,
		PubKey:           valInfo.PubKey,
		BondHeight:       0,
		UnbondHeight:     0,
		LastCommitHeight: 0,
		VotingPower:      valInfo.FirstBondAmount,
		Accum:            0,
	}
	return valInfo, val, privVal
}

func RandGenesisDoc(numAccounts int, randBalance bool, minBalance int64, numValidators int, randBonded bool, minBonded int64) (*GenesisDoc, []*account.PrivAccount, []*PrivValidator) {
	accounts := make([]GenesisAccount, numAccounts)
	privAccounts := make([]*account.PrivAccount, numAccounts)
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
	privValidators := make([]*PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		valInfo, _, privVal := RandValidator(randBonded, minBonded)
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
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return &GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     "tendermint_test",
		Accounts:    accounts,
		Validators:  validators,
	}, privAccounts, privValidators

}

func RandGenesisState(numAccounts int, randBalance bool, minBalance int64, numValidators int, randBonded bool, minBonded int64) (*State, []*account.PrivAccount, []*PrivValidator) {
	db := dbm.NewMemDB()
	genDoc, privAccounts, privValidators := RandGenesisDoc(numAccounts, randBalance, minBalance, numValidators, randBonded, minBonded)
	s0 := MakeGenesisState(db, genDoc)
	s0.Save()
	return s0, privAccounts, privValidators
}

//-------------------------------------

type PrivValidatorsByAddress []*PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address, pvs[j].Address) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
