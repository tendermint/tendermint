package state

import (
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
	. "github.com/tendermint/tendermint/db"

	"bytes"
	"testing"
	"time"
)

func randAccountDetail(id uint64, status byte) *AccountDetail {
	return &AccountDetail{
		Account: Account{
			Id:     id,
			PubKey: CRandBytes(32),
		},
		Sequence: RandUInt(),
		Balance:  RandUInt64(),
		Status:   status,
	}
}

// The first numValidators accounts are validators.
func randGenesisState(numAccounts int, numValidators int) *State {
	db := NewMemDB()
	accountDetails := make([]*AccountDetail, numAccounts)
	for i := 0; i < numAccounts; i++ {
		if i < numValidators {
			accountDetails[i] = randAccountDetail(uint64(i), AccountDetailStatusNominal)
		} else {
			accountDetails[i] = randAccountDetail(uint64(i), AccountDetailStatusBonded)
		}
	}
	s0 := GenesisState(db, time.Now(), accountDetails)
	return s0
}

func TestGenesisSaveLoad(t *testing.T) {

	// Generate a state, save & load it.
	s0 := randGenesisState(10, 5)
	// Figure out what the next state hashes should be.
	s0.Validators.Hash()
	s0ValsCopy := s0.Validators.Copy()
	s0ValsCopy.IncrementAccum()
	nextValidationStateHash := s0ValsCopy.Hash()
	nextAccountStateHash := s0.AccountDetails.Hash()
	// Mutate the state to append one empty block.
	block := &Block{
		Header: Header{
			Network:             Config.Network,
			Height:              1,
			ValidationStateHash: nextValidationStateHash,
			AccountStateHash:    nextAccountStateHash,
		},
		Data: Data{
			Txs: []Tx{},
		},
	}
	err := s0.AppendBlock(block)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}

	// Save s0
	commitTime := time.Now()
	s0.Save(commitTime)

	// Sanity check s0
	//s0.DB.(*MemDB).Print()
	if s0.Validators.TotalVotingPower() == 0 {
		t.Error("s0 Validators TotalVotingPower should not be 0")
	}
	if s0.Height != 1 {
		t.Error("s0 Height should be 1, got", s0.Height)
	}

	// Load s1
	s1 := LoadState(s0.DB)

	// Compare CommitTime
	if !s0.CommitTime.Equal(s1.CommitTime) {
		t.Error("CommitTime was not the same", s0.CommitTime, s1.CommitTime)
	}
	// Compare height & blockHash
	if s0.Height != s1.Height {
		t.Error("Height mismatch")
	}
	if !bytes.Equal(s0.BlockHash, s1.BlockHash) {
		t.Error("BlockHash mismatch")
	}
	// Compare Validators
	if s0.Validators.Size() != s1.Validators.Size() {
		t.Error("Validators Size mismatch")
	}
	if s0.Validators.TotalVotingPower() != s1.Validators.TotalVotingPower() {
		t.Error("Validators TotalVotingPower mismatch")
	}
	if !bytes.Equal(s0.AccountDetails.Hash(), s1.AccountDetails.Hash()) {
		t.Error("AccountDetail mismatch")
	}
}
