package state

import (
	"bytes"
	"sort"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"

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

func RandAccount(randBalance bool, minBalance uint64) (*Account, *PrivAccount) {
	privAccount := GenPrivAccount()
	account := &Account{
		Address:  privAccount.PubKey.Address(),
		PubKey:   privAccount.PubKey,
		Sequence: RandUint(),
		Balance:  minBalance,
	}
	if randBalance {
		account.Balance += uint64(RandUint32())
	}
	return account, privAccount
}

func RandValidator(randBonded bool, minBonded uint64) (*ValidatorInfo, *PrivValidator) {
	privVal := GenPrivValidator()
	_, privVal.filename = Tempfile("priv_validator_")
	bonded := minBonded
	if randBonded {
		bonded += uint64(RandUint32())
	}
	valInfo := &ValidatorInfo{
		Address: privVal.Address,
		PubKey:  privVal.PubKey,
		UnbondTo: []*TxOutput{&TxOutput{
			Amount:  bonded,
			Address: privVal.Address,
		}},
		FirstBondHeight: 0,
		FirstBondAmount: bonded,
	}
	return valInfo, privVal
}

func RandGenesisState(numAccounts int, randBalance bool, minBalance uint64, numValidators int, randBonded bool, minBonded uint64) (*State, []*PrivAccount, []*PrivValidator) {
	db := db_.NewMemDB()
	accounts := make([]GenesisAccount, numAccounts)
	privAccounts := make([]*PrivAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		account, privAccount := RandAccount(randBalance, minBalance)
		accounts[i] = GenesisAccount{
			Address: account.Address,
			Amount:  account.Balance,
		}
		privAccounts[i] = privAccount
	}
	validators := make([]GenesisValidator, numValidators)
	privValidators := make([]*PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		valInfo, privVal := RandValidator(randBonded, minBonded)
		validators[i] = GenesisValidator{
			PubKey: valInfo.PubKey,
			Amount: valInfo.FirstBondAmount,
			UnbondTo: []GenesisAccount{
				{
					Address: valInfo.PubKey.Address(),
					Amount:  valInfo.FirstBondAmount,
				},
			},
		}
		privValidators[i] = privVal
	}
	sort.Sort(PrivValidatorsByAddress(privValidators))
	s0 := MakeGenesisState(db, &GenesisDoc{
		GenesisTime: time.Now(),
		Accounts:    accounts,
		Validators:  validators,
	})
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
