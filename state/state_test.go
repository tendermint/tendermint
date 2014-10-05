package state

import (
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/db"

	"testing"
	"time"
)

func randAccountBalance(id uint64, status byte) *AccountBalance {
	return &AccountBalance{
		Account: Account{
			Id:     id,
			PubKey: CRandBytes(32),
		},
		Balance: RandUInt64(),
		Status:  status,
	}
}

// The first numValidators accounts are validators.
func randGenesisState(numAccounts int, numValidators int) *State {
	db := NewMemDB()
	accountBalances := make([]*AccountBalance, numAccounts)
	for i := 0; i < numAccounts; i++ {
		if i < numValidators {
			accountBalances[i] = randAccountBalance(uint64(i), AccountBalanceStatusNominal)
		} else {
			accountBalances[i] = randAccountBalance(uint64(i), AccountBalanceStatusBonded)
		}
	}
	s0 := GenesisState(db, time.Now(), accountBalances)
	return s0
}

func TestGenesisSaveLoad(t *testing.T) {

	s0 := randGenesisState(10, 5)
	t.Log(s0)

}
