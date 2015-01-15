package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	sm "github.com/tendermint/tendermint/state"
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

func getUint64(prompt string) uint64 {
	input := getString(prompt)
	i, err := strconv.Atoi(input)
	if err != nil {
		Exit(Fmt("Not a valid uint64 amount: %v\nError: %v\n", input, err))
	}
	return uint64(i)
}

func gen_tx() {

	// Get State, which may be nil.
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)

	// Get source pubkey
	srcPubKeyBytes := getByteSliceFromHex("Enter source pubkey: ")
	r, n, err := bytes.NewReader(srcPubKeyBytes), new(int64), new(error)
	srcPubKey := binary.ReadBinary(struct{ account.PubKey }{}, r, n, err).(struct{ account.PubKey }).PubKey
	if *err != nil {
		Exit(Fmt("Invalid PubKey. Error: %v", err))
	}

	// Get the state of the account.
	var srcAccount *account.Account
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
	srcSendAmount := getUint64(Fmt("Enter amount to send from %X (total: %v): ", srcAccountAddress, srcAccountBalanceStr))

	// Get the next sequence of src account
	srcSendSequence := uint(getUint64(Fmt("Enter next sequence for %X (guess: %v): ", srcAccountAddress, srcAccountSequenceStr)))

	// Get dest address
	dstAddress := getByteSliceFromHex("Enter destination address: ")

	// Get the amount to send to dst account
	dstSendAmount := getUint64(Fmt("Enter amount to send to %X: ", dstAddress))

	// Construct SendTx
	tx := &block.SendTx{
		Inputs: []*block.TxInput{
			&block.TxInput{
				Address:   srcAddress,
				Amount:    srcSendAmount,
				Sequence:  srcSendSequence,
				Signature: account.SignatureEd25519{},
				PubKey:    srcPubKey,
			},
		},
		Outputs: []*block.TxOutput{
			&block.TxOutput{
				Address: dstAddress,
				Amount:  dstSendAmount,
			},
		},
	}

	// Show the intermediate form.
	fmt.Printf("Generated tx: %X\n", binary.BinaryBytes(tx))

	// Get source privkey (for signing)
	srcPrivKeyBytes := getByteSliceFromHex("Enter source privkey (for signing): ")
	r, n, err = bytes.NewReader(srcPrivKeyBytes), new(int64), new(error)
	srcPrivKey := binary.ReadBinary(struct{ account.PrivKey }{}, r, n, err).(struct{ account.PrivKey }).PrivKey
	if *err != nil {
		Exit(Fmt("Invalid PrivKey. Error: %v", err))
	}

	// Sign
	tx.Inputs[0].Signature = srcPrivKey.Sign(account.SignBytes(tx))
	fmt.Printf("Signed tx: %X\n", binary.BinaryBytes(tx))
}
