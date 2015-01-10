package state

import (
	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	"github.com/tendermint/tendermint/config"

	"bytes"
	"testing"
	"time"
)

func TestCopyState(t *testing.T) {
	// Generate a random state
	s0, privAccounts, _ := RandGenesisState(10, true, 1000, 5, true, 1000)
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
	acc0Address := privAccounts[0].PubKey.Address()
	acc := s0.GetAccount(acc0Address)
	acc.Balance += 1

	// The account balance shouldn't have changed yet.
	if s0.GetAccount(acc0Address).Balance == acc.Balance {
		t.Error("Account balance changed unexpectedly")
	}

	// Setting, however, should change the balance.
	s0.SetAccount(acc)
	if s0.GetAccount(acc0Address).Balance != acc.Balance {
		t.Error("Account balance wasn't set")
	}

	// Now that the state changed, the hash should change too.
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
	s0, _, _ := RandGenesisState(10, true, 1000, 5, true, 1000)

	// Mutate the state to append one empty block.
	block := &Block{
		Header: &Header{
			Network:        config.App.GetString("Network"),
			Height:         1,
			Time:           s0.LastBlockTime.Add(time.Minute),
			Fees:           0,
			NumTxs:         0,
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

	// The last argument to AppendBlock() is `false`,
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
	if !bytes.Equal(s0.BondedValidators.Hash(), s1.BondedValidators.Hash()) {
		t.Error("BondedValidators hash mismatch")
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
	if !bytes.Equal(s0.accounts.Hash(), s1.accounts.Hash()) {
		t.Error("Accounts mismatch")
	}
	if !bytes.Equal(s0.validatorInfos.Hash(), s1.validatorInfos.Hash()) {
		t.Error("Accounts mismatch")
	}
}

/* TODO: Write better tests, and also refactor out common code
func TestSendTxStateSave(t *testing.T) {

	// Generate a state, save & load it.
	s0, privAccounts, _ := RandGenesisState(10, true, 1000, 5, true, 1000)

	sendTx := &SendTx{
		Inputs: []*TxInput{
			&TxInput{
				Address:   privAccounts[0].PubKey.Address(),
				Amount:    100,
				Sequence:  1,
				Signature: nil,
				PubKey:    privAccounts[0].PubKey,
			},
		},
		Outputs: []*TxOutput{
			&TxOutput{
				Address: []byte("01234567890123456789"),
				Amount:  100,
			},
		},
	}
	sendTx.Inputs[0].Signature = privAccounts[0].Sign(sendTx)

	// Mutate the state to append block with sendTx
	block := &Block{
		Header: &Header{
			Network:        config.App.GetString("Network"),
			Height:         1,
			Time:           s0.LastBlockTime.Add(time.Minute),
			Fees:           0,
			NumTxs:         1,
			LastBlockHash:  s0.LastBlockHash,
			LastBlockParts: s0.LastBlockParts,
			StateHash:      nil,
		},
		Validation: &Validation{},
		Data: &Data{
			Txs: []Tx{sendTx},
		},
	}
	blockParts := NewPartSetFromData(BinaryBytes(block))

	// The last argument to AppendBlock() is `false`,
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
	fmt.Printf("s0.accounts.Hash(): %X\n", s0.accounts.Hash())
	s0.DB.Print()

}
*/

func TestTxSequence(t *testing.T) {

	state, privAccounts, _ := RandGenesisState(3, true, 1000, 1, true, 1000)
	acc0 := state.GetAccount(privAccounts[0].PubKey.Address())
	acc0PubKey := privAccounts[0].PubKey
	acc1 := state.GetAccount(privAccounts[1].PubKey.Address())

	// Try executing a SendTx with various sequence numbers.
	makeSendTx := func(sequence uint) *SendTx {
		return &SendTx{
			Inputs: []*TxInput{
				&TxInput{
					Address:  acc0.Address,
					Amount:   1,
					Sequence: sequence,
					PubKey:   acc0PubKey,
				},
			},
			Outputs: []*TxOutput{
				&TxOutput{
					Address: acc1.Address,
					Amount:  1,
				},
			},
		}
	}

	// Test a variety of sequence numbers for the tx.
	// The tx should only pass when i == 1.
	for i := -1; i < 3; i++ {
		sequence := acc0.Sequence + uint(i)
		tx := makeSendTx(sequence)
		tx.Inputs[0].Signature = privAccounts[0].Sign(tx)
		stateCopy := state.Copy()
		err := stateCopy.ExecTx(tx)
		if i == 1 {
			// Sequence is good.
			if err != nil {
				t.Errorf("Expected good sequence to pass: %v", err)
			}
			// Check acc.Sequence.
			newAcc0 := stateCopy.GetAccount(acc0.Address)
			if newAcc0.Sequence != sequence {
				t.Errorf("Expected account sequence to change to %v, got %v",
					sequence, newAcc0.Sequence)
			}
		} else {
			// Sequence is bad.
			if err == nil {
				t.Errorf("Expected bad sequence to fail")
			}
			// Check acc.Sequence. (shouldn't have changed)
			newAcc0 := stateCopy.GetAccount(acc0.Address)
			if newAcc0.Sequence != acc0.Sequence {
				t.Errorf("Expected account sequence to not change from %v, got %v",
					acc0.Sequence, newAcc0.Sequence)
			}
		}
	}
}

// TODO: test overflows.
// TODO: test for unbonding validators.
func TestTxs(t *testing.T) {

	state, privAccounts, _ := RandGenesisState(3, true, 1000, 1, true, 1000)

	//val0 := state.GetValidatorInfo(privValidators[0].Address)
	acc0 := state.GetAccount(privAccounts[0].PubKey.Address())
	acc0PubKey := privAccounts[0].PubKey
	acc1 := state.GetAccount(privAccounts[1].PubKey.Address())

	// SendTx.
	{
		state := state.Copy()
		tx := &SendTx{
			Inputs: []*TxInput{
				&TxInput{
					Address:  acc0.Address,
					Amount:   1,
					Sequence: acc0.Sequence + 1,
					PubKey:   acc0PubKey,
				},
			},
			Outputs: []*TxOutput{
				&TxOutput{
					Address: acc1.Address,
					Amount:  1,
				},
			},
		}

		tx.Inputs[0].Signature = privAccounts[0].Sign(tx)
		err := state.ExecTx(tx)
		if err != nil {
			t.Errorf("Got error in executing send transaction, %v", err)
		}
		newAcc0 := state.GetAccount(acc0.Address)
		if acc0.Balance-1 != newAcc0.Balance {
			t.Errorf("Unexpected newAcc0 balance. Expected %v, got %v",
				acc0.Balance-1, newAcc0.Balance)
		}
		newAcc1 := state.GetAccount(acc1.Address)
		if acc1.Balance+1 != newAcc1.Balance {
			t.Errorf("Unexpected newAcc1 balance. Expected %v, got %v",
				acc1.Balance+1, newAcc1.Balance)
		}
	}

	// BondTx.
	{
		state := state.Copy()
		tx := &BondTx{
			PubKey: acc0PubKey.(PubKeyEd25519),
			Inputs: []*TxInput{
				&TxInput{
					Address:  acc0.Address,
					Amount:   1,
					Sequence: acc0.Sequence + 1,
					PubKey:   acc0PubKey,
				},
			},
			UnbondTo: []*TxOutput{
				&TxOutput{
					Address: acc0.Address,
					Amount:  1,
				},
			},
		}
		tx.Inputs[0].Signature = privAccounts[0].Sign(tx)
		err := state.ExecTx(tx)
		if err != nil {
			t.Errorf("Got error in executing bond transaction, %v", err)
		}
		newAcc0 := state.GetAccount(acc0.Address)
		if newAcc0.Balance != acc0.Balance-1 {
			t.Errorf("Unexpected newAcc0 balance. Expected %v, got %v",
				acc0.Balance-1, newAcc0.Balance)
		}
		_, acc0Val := state.BondedValidators.GetByAddress(acc0.Address)
		if acc0Val == nil {
			t.Errorf("acc0Val not present")
		}
		if acc0Val.BondHeight != state.LastBlockHeight+1 {
			t.Errorf("Unexpected bond height. Expected %v, got %v",
				state.LastBlockHeight, acc0Val.BondHeight)
		}
		if acc0Val.VotingPower != 1 {
			t.Errorf("Unexpected voting power. Expected %v, got %v",
				acc0Val.VotingPower, acc0.Balance)
		}
		if acc0Val.Accum != 0 {
			t.Errorf("Unexpected accum. Expected 0, got %v",
				acc0Val.Accum)
		}
	}

	// TODO UnbondTx.

}
