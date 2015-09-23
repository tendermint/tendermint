package state

import (
	"bytes"
	"testing"
	"time"

	"github.com/tendermint/tendermint/account"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
)

func execTxWithState(state *State, tx types.Tx, runCall bool) error {
	cache := NewBlockCache(state)
	if err := ExecTx(cache, tx, runCall, nil); err != nil {
		return err
	} else {
		cache.Sync()
		return nil
	}
}

func execTxWithStateNewBlock(state *State, tx types.Tx, runCall bool) error {
	if err := execTxWithState(state, tx, runCall); err != nil {
		return err
	}

	state.LastBlockHeight += 1
	return nil
}

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
	s0.UpdateAccount(acc)
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

func makeBlock(t *testing.T, state *State, validation *types.Validation, txs []types.Tx) *types.Block {
	if validation == nil {
		validation = &types.Validation{}
	}
	block := &types.Block{
		Header: &types.Header{
			ChainID:        state.ChainID,
			Height:         state.LastBlockHeight + 1,
			Time:           state.LastBlockTime.Add(time.Minute),
			Fees:           0,
			NumTxs:         len(txs),
			LastBlockHash:  state.LastBlockHash,
			LastBlockParts: state.LastBlockParts,
			StateHash:      nil,
		},
		LastValidation: validation,
		Data: &types.Data{
			Txs: txs,
		},
	}
	block.FillHeader()

	// Fill in block StateHash
	err := state.ComputeBlockStateHash(block)
	if err != nil {
		t.Error("Error appending initial block:", err)
	}
	if len(block.Header.StateHash) == 0 {
		t.Error("Expected StateHash but got nothing.")
	}

	return block
}

func TestGenesisSaveLoad(t *testing.T) {

	// Generate a state, save & load it.
	s0, _, _ := RandGenesisState(10, true, 1000, 5, true, 1000)

	// Make complete block and blockParts
	block := makeBlock(t, s0, nil, nil)
	blockParts := block.MakePartSet()

	// Now append the block to s0.
	err := ExecBlock(s0, block, blockParts.Header())
	if err != nil {
		t.Error("Error appending initial block:", err)
	}

	// Save s0
	s0.Save()

	// Sanity check s0
	//s0.DB.(*dbm.MemDB).Print()
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

func TestTxSequence(t *testing.T) {

	state, privAccounts, _ := RandGenesisState(3, true, 1000, 1, true, 1000)
	acc0 := state.GetAccount(privAccounts[0].PubKey.Address())
	acc0PubKey := privAccounts[0].PubKey
	acc1 := state.GetAccount(privAccounts[1].PubKey.Address())

	// Test a variety of sequence numbers for the tx.
	// The tx should only pass when i == 1.
	for i := -1; i < 3; i++ {
		sequence := acc0.Sequence + i
		tx := types.NewSendTx()
		tx.AddInputWithNonce(acc0PubKey, 1, sequence)
		tx.AddOutput(acc1.Address, 1)
		tx.Inputs[0].Signature = privAccounts[0].Sign(state.ChainID, tx)
		stateCopy := state.Copy()
		err := execTxWithState(stateCopy, tx, true)
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

func TestNameTxs(t *testing.T) {
	state, privAccounts, _ := RandGenesisState(3, true, 1000, 1, true, 1000)

	types.MinNameRegistrationPeriod = 5
	startingBlock := state.LastBlockHeight

	// try some bad names. these should all fail
	names := []string{"", "\n", "123#$%", "\x00", string([]byte{20, 40, 60, 80}), "baffledbythespectacleinallofthisyouseeehesaidwithouteyessurprised", "no spaces please"}
	data := "something about all this just doesn't feel right."
	fee := int64(1000)
	numDesiredBlocks := 5
	for _, name := range names {
		amt := fee + int64(numDesiredBlocks)*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)
		tx, _ := types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
		tx.Sign(state.ChainID, privAccounts[0])

		if err := execTxWithState(state, tx, true); err == nil {
			t.Fatalf("Expected invalid name error from %s", name)
		}
	}

	// try some bad data. these should all fail
	name := "hold_it_chum"
	datas := []string{"cold&warm", "!@#$%^&*()", "<<<>>>>", "because why would you ever need a ~ or a & or even a % in a json file? make your case and we'll talk"}
	for _, data := range datas {
		amt := fee + int64(numDesiredBlocks)*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)
		tx, _ := types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
		tx.Sign(state.ChainID, privAccounts[0])

		if err := execTxWithState(state, tx, true); err == nil {
			t.Fatalf("Expected invalid data error from %s", data)
		}
	}

	validateEntry := func(t *testing.T, entry *types.NameRegEntry, name, data string, addr []byte, expires int) {

		if entry == nil {
			t.Fatalf("Could not find name %s", name)
		}
		if bytes.Compare(entry.Owner, addr) != 0 {
			t.Fatalf("Wrong owner. Got %X expected %X", entry.Owner, addr)
		}
		if data != entry.Data {
			t.Fatalf("Wrong data. Got %s expected %s", entry.Data, data)
		}
		if name != entry.Name {
			t.Fatalf("Wrong name. Got %s expected %s", entry.Name, name)
		}
		if expires != entry.Expires {
			t.Fatalf("Wrong expiry. Got %d, expected %d", entry.Expires, expires)
		}
	}

	// try a good one, check data, owner, expiry
	name = "@looking_good/karaoke_bar.broadband"
	data = "on this side of neptune there are 1234567890 people: first is OMNIVORE+-3. Or is it. Ok this is pretty restrictive. No exclamations :(. Faces tho :')"
	amt := fee + int64(numDesiredBlocks)*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)
	tx, _ := types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[0])
	if err := execTxWithState(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry := state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[0].Address, startingBlock+numDesiredBlocks)

	// fail to update it as non-owner, in same block
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithState(state, tx, true); err == nil {
		t.Fatal("Expected error")
	}

	// update it as owner, just to increase expiry, in same block
	// NOTE: we have to resend the data or it will clear it (is this what we want?)
	tx, _ = types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[0])
	if err := execTxWithStateNewBlock(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[0].Address, startingBlock+numDesiredBlocks*2)

	// update it as owner, just to increase expiry, in next block
	tx, _ = types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[0])
	if err := execTxWithStateNewBlock(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[0].Address, startingBlock+numDesiredBlocks*3)

	// fail to update it as non-owner
	state.LastBlockHeight = entry.Expires - 1
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithState(state, tx, true); err == nil {
		t.Fatal("Expected error")
	}

	// once expires, non-owner succeeds
	state.LastBlockHeight = entry.Expires
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithState(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[1].Address, state.LastBlockHeight+numDesiredBlocks)

	// update it as new owner, with new data (longer), but keep the expiry!
	data = "In the beginning there was no thing, not even the beginning. It hadn't been here, no there, nor for that matter anywhere, not especially because it had not to even exist, let alone to not. Nothing especially odd about that."
	oldCredit := amt - fee
	numDesiredBlocks = 10
	amt = fee + (int64(numDesiredBlocks)*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data) - oldCredit)
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithState(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[1].Address, state.LastBlockHeight+numDesiredBlocks)

	// test removal
	amt = fee
	data = ""
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithStateNewBlock(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	if entry != nil {
		t.Fatal("Expected removed entry to be nil")
	}

	// create entry by key0,
	// test removal by key1 after expiry
	name = "looking_good/karaoke_bar"
	data = "some data"
	amt = fee + int64(numDesiredBlocks)*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)
	tx, _ = types.NewNameTx(state, privAccounts[0].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[0])
	if err := execTxWithState(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	validateEntry(t, entry, name, data, privAccounts[0].Address, state.LastBlockHeight+numDesiredBlocks)
	state.LastBlockHeight = entry.Expires

	amt = fee
	data = ""
	tx, _ = types.NewNameTx(state, privAccounts[1].PubKey, name, data, amt, fee)
	tx.Sign(state.ChainID, privAccounts[1])
	if err := execTxWithStateNewBlock(state, tx, true); err != nil {
		t.Fatal(err)
	}
	entry = state.GetNameRegEntry(name)
	if entry != nil {
		t.Fatal("Expected removed entry to be nil")
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
		tx := &types.SendTx{
			Inputs: []*types.TxInput{
				&types.TxInput{
					Address:  acc0.Address,
					Amount:   1,
					Sequence: acc0.Sequence + 1,
					PubKey:   acc0PubKey,
				},
			},
			Outputs: []*types.TxOutput{
				&types.TxOutput{
					Address: acc1.Address,
					Amount:  1,
				},
			},
		}

		tx.Inputs[0].Signature = privAccounts[0].Sign(state.ChainID, tx)
		err := execTxWithState(state, tx, true)
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

	// CallTx. Just runs through it and checks the transfer. See vm, rpc tests for more
	{
		state := state.Copy()
		newAcc1 := state.GetAccount(acc1.Address)
		newAcc1.Code = []byte{0x60}
		state.UpdateAccount(newAcc1)
		tx := &types.CallTx{
			Input: &types.TxInput{
				Address:  acc0.Address,
				Amount:   1,
				Sequence: acc0.Sequence + 1,
				PubKey:   acc0PubKey,
			},
			Address:  acc1.Address,
			GasLimit: 10,
		}

		tx.Input.Signature = privAccounts[0].Sign(state.ChainID, tx)
		err := execTxWithState(state, tx, true)
		if err != nil {
			t.Errorf("Got error in executing call transaction, %v", err)
		}
		newAcc0 := state.GetAccount(acc0.Address)
		if acc0.Balance-1 != newAcc0.Balance {
			t.Errorf("Unexpected newAcc0 balance. Expected %v, got %v",
				acc0.Balance-1, newAcc0.Balance)
		}
		newAcc1 = state.GetAccount(acc1.Address)
		if acc1.Balance+1 != newAcc1.Balance {
			t.Errorf("Unexpected newAcc1 balance. Expected %v, got %v",
				acc1.Balance+1, newAcc1.Balance)
		}
	}

	// NameTx.
	{
		entryName := "satoshi"
		entryData := `
A  purely   peer-to-peer   version   of   electronic   cash   would   allow   online
payments  to  be  sent   directly  from  one  party  to  another  without   going  through  a
financial institution.   Digital signatures provide part of the solution, but the main
benefits are lost if a trusted third party is still required to prevent double-spending.
We propose a solution to the double-spending problem using a peer-to-peer network.
The   network   timestamps   transactions  by  hashing   them   into   an   ongoing   chain   of
hash-based proof-of-work, forming a record that cannot be changed without redoing
the proof-of-work.   The longest chain not only serves as proof of the sequence of
events witnessed, but proof that it came from the largest pool of CPU power.   As
long as a majority of CPU power is controlled by nodes that are not cooperating to
attack the network, they'll generate the longest chain and outpace attackers.   The
network itself requires minimal structure.   Messages are broadcast on a best effort
basis,   and   nodes   can   leave  and   rejoin   the  network   at  will,  accepting   the   longest
proof-of-work chain as proof of what happened while they were gone `
		entryAmount := int64(10000)

		state := state.Copy()
		tx := &types.NameTx{
			Input: &types.TxInput{
				Address:  acc0.Address,
				Amount:   entryAmount,
				Sequence: acc0.Sequence + 1,
				PubKey:   acc0PubKey,
			},
			Name: entryName,
			Data: entryData,
		}

		tx.Input.Signature = privAccounts[0].Sign(state.ChainID, tx)
		err := execTxWithState(state, tx, true)
		if err != nil {
			t.Errorf("Got error in executing call transaction, %v", err)
		}
		newAcc0 := state.GetAccount(acc0.Address)
		if acc0.Balance-entryAmount != newAcc0.Balance {
			t.Errorf("Unexpected newAcc0 balance. Expected %v, got %v",
				acc0.Balance-entryAmount, newAcc0.Balance)
		}
		entry := state.GetNameRegEntry(entryName)
		if entry == nil {
			t.Errorf("Expected an entry but got nil")
		}
		if entry.Data != entryData {
			t.Errorf("Wrong data stored")
		}

		// test a bad string
		tx.Data = string([]byte{0, 1, 2, 3, 127, 128, 129, 200, 251})
		tx.Input.Sequence += 1
		tx.Input.Signature = privAccounts[0].Sign(state.ChainID, tx)
		err = execTxWithState(state, tx, true)
		if _, ok := err.(types.ErrTxInvalidString); !ok {
			t.Errorf("Expected invalid string error. Got: %s", err.Error())
		}
	}

	// BondTx.
	{
		state := state.Copy()
		tx := &types.BondTx{
			PubKey: acc0PubKey.(account.PubKeyEd25519),
			Inputs: []*types.TxInput{
				&types.TxInput{
					Address:  acc0.Address,
					Amount:   1,
					Sequence: acc0.Sequence + 1,
					PubKey:   acc0PubKey,
				},
			},
			UnbondTo: []*types.TxOutput{
				&types.TxOutput{
					Address: acc0.Address,
					Amount:  1,
				},
			},
		}
		tx.Signature = privAccounts[0].Sign(state.ChainID, tx).(account.SignatureEd25519)
		tx.Inputs[0].Signature = privAccounts[0].Sign(state.ChainID, tx)
		err := execTxWithState(state, tx, true)
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

func TestSuicide(t *testing.T) {

	state, privAccounts, _ := RandGenesisState(3, true, 1000, 1, true, 1000)

	acc0 := state.GetAccount(privAccounts[0].PubKey.Address())
	acc0PubKey := privAccounts[0].PubKey
	acc1 := state.GetAccount(privAccounts[1].PubKey.Address())
	acc2 := state.GetAccount(privAccounts[2].Address)
	sendingAmount, refundedBalance, oldBalance := int64(1), acc1.Balance, acc2.Balance

	newAcc1 := state.GetAccount(acc1.Address)

	// store 0x1 at 0x1, push an address, then suicide :)
	contractCode := []byte{0x60, 0x01, 0x60, 0x01, 0x55, 0x73}
	contractCode = append(contractCode, acc2.Address...)
	contractCode = append(contractCode, 0xff)
	newAcc1.Code = contractCode
	state.UpdateAccount(newAcc1)

	// send call tx with no data, cause suicide
	tx := types.NewCallTxWithNonce(acc0PubKey, acc1.Address, nil, sendingAmount, 1000, 0, acc0.Sequence+1)
	tx.Input.Signature = privAccounts[0].Sign(state.ChainID, tx)

	// we use cache instead of execTxWithState so we can run the tx twice
	cache := NewBlockCache(state)
	if err := ExecTx(cache, tx, true, nil); err != nil {
		t.Errorf("Got error in executing call transaction, %v", err)
	}

	// if we do it again, we won't get an error, but the suicide
	// shouldn't happen twice and the caller should lose fee
	tx.Input.Sequence += 1
	tx.Input.Signature = privAccounts[0].Sign(state.ChainID, tx)
	if err := ExecTx(cache, tx, true, nil); err != nil {
		t.Errorf("Got error in executing call transaction, %v", err)
	}

	// commit the block
	cache.Sync()

	// acc2 should receive the sent funds and the contracts balance
	newAcc2 := state.GetAccount(acc2.Address)
	newBalance := sendingAmount + refundedBalance + oldBalance
	if newAcc2.Balance != newBalance {
		t.Errorf("Unexpected newAcc2 balance. Expected %v, got %v",
			newAcc2.Balance, newBalance)
	}
	newAcc1 = state.GetAccount(acc1.Address)
	if newAcc1 != nil {
		t.Errorf("Expected account to be removed")
	}

}

func TestAddValidator(t *testing.T) {

	// Generate a state, save & load it.
	s0, privAccounts, privValidators := RandGenesisState(10, false, 1000, 1, false, 1000)

	// The first privAccount will become a validator
	acc0 := privAccounts[0]
	bondTx := &types.BondTx{
		PubKey: acc0.PubKey.(account.PubKeyEd25519),
		Inputs: []*types.TxInput{
			&types.TxInput{
				Address:  acc0.Address,
				Amount:   1000,
				Sequence: 1,
				PubKey:   acc0.PubKey,
			},
		},
		UnbondTo: []*types.TxOutput{
			&types.TxOutput{
				Address: acc0.Address,
				Amount:  1000,
			},
		},
	}
	bondTx.Signature = acc0.Sign(s0.ChainID, bondTx).(account.SignatureEd25519)
	bondTx.Inputs[0].Signature = acc0.Sign(s0.ChainID, bondTx)

	// Make complete block and blockParts
	block0 := makeBlock(t, s0, nil, []types.Tx{bondTx})
	block0Parts := block0.MakePartSet()

	// Sanity check
	if s0.BondedValidators.Size() != 1 {
		t.Error("Expected there to be 1 validators before bondTx")
	}

	// Now append the block to s0.
	err := ExecBlock(s0, block0, block0Parts.Header())
	if err != nil {
		t.Error("Error appending initial block:", err)
	}

	// Must save before further modification
	s0.Save()

	// Test new validator set
	if s0.BondedValidators.Size() != 2 {
		t.Error("Expected there to be 2 validators after bondTx")
	}

	// The validation for the next block should only require 1 signature
	// (the new validator wasn't active for block0)
	precommit0 := &types.Vote{
		Height:           1,
		Round:            0,
		Type:             types.VoteTypePrecommit,
		BlockHash:        block0.Hash(),
		BlockPartsHeader: block0Parts.Header(),
	}
	privValidators[0].SignVote(s0.ChainID, precommit0)

	block1 := makeBlock(t, s0,
		&types.Validation{
			Precommits: []*types.Vote{
				precommit0,
			},
		}, nil,
	)
	block1Parts := block1.MakePartSet()
	err = ExecBlock(s0, block1, block1Parts.Header())
	if err != nil {
		t.Error("Error appending secondary block:", err)
	}
}
