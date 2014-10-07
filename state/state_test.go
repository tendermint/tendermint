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

	// Generate a state, save & load it.
	s0 := randGenesisState(10, 5)
	// Mutate the state to append one block.
	block := &Block{Data: Data{Txs: []Tx{}}}
	s0.AppendBlock(block)

	// Save s0, load s1.
	commitTime := time.Now()
	s0.Save(commitTime)
	// s0.db.(*MemDB).Print()
	s1 := LoadState(s0.db)

	// Compare CommitTime
	if commitTime.Unix() != s1.CommitTime().Unix() {
		t.Error("CommitTime was not the same")
	}
	// Compare height & blockHash
	// XXX
	// Compare Validators
	s0Vals := s0.Validators()
	s1Vals := s1.Validators()
	if s0Vals.Size() != s1Vals.Size() {
		t.Error("Validators Size changed")
	}
	if s0Vals.TotalVotingPower() == 0 {
		t.Error("s0 Validators TotalVotingPower should not be 0")
	}
	if s0Vals.TotalVotingPower() != s1Vals.TotalVotingPower() {
		t.Error("Validators TotalVotingPower changed")
	}
	// TODO Compare accountBalances, height, blockHash

}
