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
			accountDetails[i] = randAccountDetail(uint64(i), AccountStatusNominal)
		} else {
			accountDetails[i] = randAccountDetail(uint64(i), AccountStatusBonded)
		}
	}
	s0 := GenesisState(db, time.Now(), accountDetails)
	s0.Save(time.Now())
	return s0
}

func TestCopyState(t *testing.T) {
	// Generate a state
	s0 := randGenesisState(10, 5)
	s0Hash := s0.Hash()
	if len(s0Hash) == 0 {
		t.Error("Expected state hash")
	}

	// Check hash of copy
	s0Copy := s0.Copy()
	if !bytes.Equal(s0Hash, s0Copy.Hash()) {
		t.Error("Expected state copy hash to be the same")
	}

	// Mutate the original.
	_, accDet_ := s0.AccountDetails.GetByIndex(0)
	accDet := accDet_.(*AccountDetail)
	if accDet == nil {
		t.Error("Expected state to have an account")
	}
	accDet.Balance += 1
	s0.AccountDetails.Set(accDet.Id, accDet)
	if bytes.Equal(s0Hash, s0.Hash()) {
		t.Error("Expected state hash to have changed")
	}
	if !bytes.Equal(s0Hash, s0Copy.Hash()) {
		t.Error("Expected state copy hash to have not changed")
	}
}

func TestGenesisSaveLoad(t *testing.T) {

	// Generate a state, save & load it.
	s0 := randGenesisState(10, 5)
	// Mutate the state to append one empty block.
	block := &Block{
		Header: Header{
			Network:   Config.Network,
			Height:    1,
			StateHash: nil,
		},
		Data: Data{
			Txs: []Tx{},
		},
	}
	// The second argument to AppendBlock() is false,
	// which sets Block.Header.StateHash.
	err := s0.Copy().AppendBlock(block, false)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}
	if len(block.Header.StateHash) == 0 {
		t.Error("Expected StateHash but got nothing.")
	}
	// Now append the block to s0.
	// This time we also check the StateHash (as computed above).
	err = s0.AppendBlock(block, true)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}

	// Save s0
	commitTime := time.Now()
	s0.Save(commitTime)

	// Sanity check s0
	//s0.DB.(*MemDB).Print()
	if s0.BondedValidators.TotalVotingPower() == 0 {
		t.Error("s0 BondedValidators TotalVotingPower should not be 0")
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
	// Compare state merkle trees
	if s0.BondedValidators.Size() != s1.BondedValidators.Size() {
		t.Error("BondedValidators Size mismatch")
	}
	if s0.BondedValidators.TotalVotingPower() != s1.BondedValidators.TotalVotingPower() {
		t.Error("BondedValidators TotalVotingPower mismatch")
	}
	if bytes.Equal(s0.BondedValidators.Hash(), s1.BondedValidators.Hash()) {
		// The BondedValidators hash should have changed because
		// each AppendBlock() calls IncrementAccum(),
		// changing each validator's Accum.
		t.Error("BondedValidators hash should have changed")
	}
	if s0.UnbondingValidators.Size() != s1.UnbondingValidators.Size() {
		t.Error("UnbondingValidators Size mismatch")
	}
	if s0.UnbondingValidators.TotalVotingPower() != s1.UnbondingValidators.TotalVotingPower() {
		t.Error("UnbondingValidators TotalVotingPower mismatch")
	}
	if !bytes.Equal(s0.UnbondingValidators.Hash(), s1.UnbondingValidators.Hash()) {
		t.Error("UnbondingValidators hash mismatch")
	}
	if !bytes.Equal(s0.AccountDetails.Hash(), s1.AccountDetails.Hash()) {
		t.Error("AccountDetail mismatch")
	}
}
