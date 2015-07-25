package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/wire"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func getString(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return input[:len(input)-1]
}

func getByteSliceFromHex(prompt string) []byte {
	input := getString(prompt)
	bytes, err := hex.DecodeString(input)
	if err != nil {
		Exit(Fmt("Not in hex format: %v\nError: %v\n", input, err))
	}
	return bytes
}

func getInt(prompt string) int {
	input := getString(prompt)
	i, err := strconv.Atoi(input)
	if err != nil {
		Exit(Fmt("Not a valid int64 amount: %v\nError: %v\n", input, err))
	}
	return i
}

func getInt64(prompt string) int64 {
	return int64(getInt(prompt))
}

func gen_tx() {

	// Get State, which may be nil.
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)

	// Get source pubkey
	srcPubKeyBytes := getByteSliceFromHex("Enter source pubkey: ")
	r, n, err := bytes.NewReader(srcPubKeyBytes), new(int64), new(error)
	srcPubKey := wire.ReadBinary(struct{ acm.PubKey }{}, r, n, err).(struct{ acm.PubKey }).PubKey
	if *err != nil {
		Exit(Fmt("Invalid PubKey. Error: %v", err))
	}

	// Get the state of the account.
	var srcAccount *acm.Account
	var srcAccountAddress = srcPubKey.Address()
	var srcAccountBalanceStr = "unknown"
	var srcAccountSequenceStr = "unknown"
	srcAddress := srcPubKey.Address()
	if state != nil {
		srcAccount = state.GetAccount(srcAddress)
		srcAccountBalanceStr = Fmt("%v", srcAccount.Balance)
		srcAccountSequenceStr = Fmt("%v", srcAccount.Sequence+1)
	}

	// Get the amount to send from src account
	srcSendAmount := getInt64(Fmt("Enter amount to send from %X (total: %v): ", srcAccountAddress, srcAccountBalanceStr))

	// Get the next sequence of src account
	srcSendSequence := getInt(Fmt("Enter next sequence for %X (guess: %v): ", srcAccountAddress, srcAccountSequenceStr))

	// Get dest address
	dstAddress := getByteSliceFromHex("Enter destination address: ")

	// Get the amount to send to dst account
	dstSendAmount := getInt64(Fmt("Enter amount to send to %X: ", dstAddress))

	// Construct SendTx
	tx := &types.SendTx{
		Inputs: []*types.TxInput{
			&types.TxInput{
				Address:   srcAddress,
				Amount:    srcSendAmount,
				Sequence:  srcSendSequence,
				Signature: acm.SignatureEd25519{},
				PubKey:    srcPubKey,
			},
		},
		Outputs: []*types.TxOutput{
			&types.TxOutput{
				Address: dstAddress,
				Amount:  dstSendAmount,
			},
		},
	}

	// Show the intermediate form.
	fmt.Printf("Generated tx: %X\n", wire.BinaryBytes(tx))

	// Get source privkey (for signing)
	srcPrivKeyBytes := getByteSliceFromHex("Enter source privkey (for signing): ")
	r, n, err = bytes.NewReader(srcPrivKeyBytes), new(int64), new(error)
	srcPrivKey := wire.ReadBinary(struct{ acm.PrivKey }{}, r, n, err).(struct{ acm.PrivKey }).PrivKey
	if *err != nil {
		Exit(Fmt("Invalid PrivKey. Error: %v", err))
	}

	// Sign
	tx.Inputs[0].Signature = srcPrivKey.Sign(acm.SignBytes(config.GetString("chain_id"), tx))
	fmt.Printf("Signed tx: %X\n", wire.BinaryBytes(tx))
}
