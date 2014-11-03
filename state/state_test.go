package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
	db_ "github.com/tendermint/tendermint/db"

	"bytes"
	"testing"
	"time"
)

func randAccountDetail(id uint64, status byte) (*AccountDetail, *PrivAccount) {
	privAccount := GenPrivAccount()
	privAccount.Id = id
	account := privAccount.Account
	return &AccountDetail{
		Account:  account,
		Sequence: RandUInt(),
		Balance:  RandUInt64() + 1000, // At least 1000.
		Status:   status,
	}, privAccount
}

// The first numValidators accounts are validators.
func randGenesisState(numAccounts int, numValidators int) (*State, []*PrivAccount) {
	db := db_.NewMemDB()
	accountDetails := make([]*AccountDetail, numAccounts)
	privAccounts := make([]*PrivAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		if i < numValidators {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), AccountStatusBonded)
		} else {
			accountDetails[i], privAccounts[i] =
				randAccountDetail(uint64(i), AccountStatusNominal)
		}
	}
	s0 := GenesisState(db, time.Now(), accountDetails)
	s0.Save()
	return s0, privAccounts
}

func TestCopyState(t *testing.T) {
	// Generate a state
	s0, _ := randGenesisState(10, 5)
	s0Hash := s0.Hash()
	if len(s0Hash) == 0 {
		t.Error("Expected state hash")
	}

	// Check hash of copy
	s0Copy := s0.Copy()
	if !bytes.Equal(s0Hash, s0Copy.Hash()) {
		t.Error("Expected state copy hash to be the same")
	}

	// Mutate the original; hash should change.
	accDet := s0.GetAccountDetail(0)
	accDet.Balance += 1
	// The account balance shouldn't have changed yet.
	if s0.GetAccountDetail(0).Balance == accDet.Balance {
		t.Error("Account balance changed unexpectedly")
	}
	// Setting, however, should change the balance.
	s0.SetAccountDetail(accDet)
	if s0.GetAccountDetail(0).Balance != accDet.Balance {
		t.Error("Account balance wasn't set")
	}
	// How that the state changed, the hash should change too.
	if bytes.Equal(s0Hash, s0.Hash()) {
		t.Error("Expected state hash to have changed")
	}
	// The s0Copy shouldn't have changed though.
	if !bytes.Equal(s0Hash, s0Copy.Hash()) {
		t.Error("Expected state copy hash to have not changed")
	}
}

func TestGenesisSaveLoad(t *testing.T) {

	// Generate a state, save & load it.
	s0, _ := randGenesisState(10, 5)
	// Mutate the state to append one empty block.
	block := &Block{
		Header: &Header{
			Network:        Config.Network,
			Height:         1,
			Time:           s0.LastBlockTime.Add(time.Minute),
			Fees:           0,
			LastBlockHash:  s0.LastBlockHash,
			LastBlockParts: s0.LastBlockParts,
			StateHash:      nil,
		},
		Validation: &Validation{},
		Data: &Data{
			Txs: []Tx{},
		},
	}
	blockParts := NewPartSetFromData(BinaryBytes(block))
	// The second argument to AppendBlock() is false,
	// which sets Block.Header.StateHash.
	err := s0.Copy().AppendBlock(block, blockParts.Header(), false)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}
	if len(block.Header.StateHash) == 0 {
		t.Error("Expected StateHash but got nothing.")
	}
	// Now append the block to s0.
	// This time we also check the StateHash (as computed above).
	err = s0.AppendBlock(block, blockParts.Header(), true)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}

	// Save s0
	s0.Save()

	// Sanity check s0
	//s0.DB.(*db_.MemDB).Print()
	if s0.BondedValidators.TotalVotingPower() == 0 {
		t.Error("s0 BondedValidators TotalVotingPower should not be 0")
	}
	if s0.LastBlockHeight != 1 {
		t.Error("s0 LastBlockHeight should be 1, got", s0.LastBlockHeight)
	}

	// Load s1
	s1 := LoadState(s0.DB)

	// Compare height & blockHash
	if s0.LastBlockHeight != s1.LastBlockHeight {
		t.Error("LastBlockHeight mismatch")
	}
	if !bytes.Equal(s0.LastBlockHash, s1.LastBlockHash) {
		t.Error("LastBlockHash mismatch")
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
	if !bytes.Equal(s0.accountDetails.Hash(), s1.accountDetails.Hash()) {
		t.Error("AccountDetail mismatch")
	}
}

func TestTxSequence(t *testing.T) {

	state, privAccounts := randGenesisState(3, 1)
	acc1 := state.GetAccountDetail(1) // Non-validator

	// Try executing a SendTx with various sequence numbers.
	stxProto := SendTx{
		BaseTx: BaseTx{
			Sequence: acc1.Sequence + 1,
			Fee:      0},
		To:     2,
		Amount: 1,
	}

	// Test a variety of sequence numbers for the tx.
	// The tx should only pass when i == 1.
	for i := -1; i < 3; i++ {
		stxCopy := stxProto
		stx := &stxCopy
		stx.Sequence = uint(int(acc1.Sequence) + i)
		privAccounts[1].Sign(stx)
		stateCopy := state.Copy()
		err := stateCopy.ExecTx(stx)
		if i >= 1 {
			// Sequence is good.
			if err != nil {
				t.Errorf("Expected good sequence to pass")
			}
			// Check accDet.Sequence.
			newAcc1 := stateCopy.GetAccountDetail(1)
			if newAcc1.Sequence != stx.Sequence {
				t.Errorf("Expected account sequence to change")
			}
		} else {
			// Sequence is bad.
			if err == nil {
				t.Errorf("Expected bad sequence to fail")
			}
			// Check accDet.Sequence. (shouldn't have changed)
			newAcc1 := stateCopy.GetAccountDetail(1)
			if newAcc1.Sequence != acc1.Sequence {
				t.Errorf("Expected account sequence to not change")
			}
		}
	}
}

func TestTxs(t *testing.T) {

	state, privAccounts := randGenesisState(3, 1)

	acc0 := state.GetAccountDetail(0) // Validator
	acc1 := state.GetAccountDetail(1) // Non-validator
	acc2 := state.GetAccountDetail(2) // Non-validator

	// SendTx.
	{
		state := state.Copy()
		stx := &SendTx{
			BaseTx: BaseTx{
				Sequence: acc1.Sequence + 1,
				Fee:      0},
			To:     2,
			Amount: 1,
		}
		privAccounts[1].Sign(stx)
		err := state.ExecTx(stx)
		if err != nil {
			t.Errorf("Got error in executing send transaction, %v", err)
		}
		newAcc1 := state.GetAccountDetail(1)
		if acc1.Balance-1 != newAcc1.Balance {
			t.Errorf("Unexpected newAcc1 balance. Expected %v, got %v",
				acc1.Balance-1, newAcc1.Balance)
		}
		newAcc2 := state.GetAccountDetail(2)
		if acc2.Balance+1 != newAcc2.Balance {
			t.Errorf("Unexpected newAcc2 balance. Expected %v, got %v",
				acc2.Balance+1, newAcc2.Balance)
		}
	}

	// TODO: test overflows.

	// SendTx should fail for bonded validators.
	{
		state := state.Copy()
		stx := &SendTx{
			BaseTx: BaseTx{
				Sequence: acc0.Sequence + 1,
				Fee:      0},
			To:     2,
			Amount: 1,
		}
		privAccounts[0].Sign(stx)
		err := state.ExecTx(stx)
		if err == nil {
			t.Errorf("Expected error, SendTx should fail for bonded validators")
		}
	}

	// TODO: test for unbonding validators.

	// BondTx.
	{
		state := state.Copy()
		btx := &BondTx{
			BaseTx: BaseTx{
				Sequence: acc1.Sequence + 1,
				Fee:      0},
		}
		privAccounts[1].Sign(btx)
		err := state.ExecTx(btx)
		if err != nil {
			t.Errorf("Got error in executing bond transaction, %v", err)
		}
		newAcc1 := state.GetAccountDetail(1)
		if acc1.Balance != newAcc1.Balance {
			t.Errorf("Unexpected newAcc1 balance. Expected %v, got %v",
				acc1.Balance, newAcc1.Balance)
		}
		if newAcc1.Status != AccountStatusBonded {
			t.Errorf("Unexpected newAcc1 status.")
		}
		_, acc1Val := state.BondedValidators.GetById(acc1.Id)
		if acc1Val == nil {
			t.Errorf("acc1Val not present")
		}
		if acc1Val.BondHeight != state.LastBlockHeight {
			t.Errorf("Unexpected bond height. Expected %v, got %v",
				state.LastBlockHeight, acc1Val.BondHeight)
		}
		if acc1Val.VotingPower != acc1.Balance {
			t.Errorf("Unexpected voting power. Expected %v, got %v",
				acc1Val.VotingPower, acc1.Balance)
		}
		if acc1Val.Accum != 0 {
			t.Errorf("Unexpected accum. Expected 0, got %v",
				acc1Val.Accum)
		}
	}

	// TODO UnbondTx.
	// TODO NameTx.

}
