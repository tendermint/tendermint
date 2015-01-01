package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	account_ "github.com/tendermint/tendermint/account"
	binary "github.com/tendermint/tendermint/binary"
	block_ "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	state_ "github.com/tendermint/tendermint/state"
)

func getString(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return input[:len(input)-1]
}

func getByteSliceFromBase64(prompt string) []byte {
	input := getString(prompt)
	bytes, err := hex.DecodeString(input)
	if err != nil {
		Exit(Fmt("Not in hexadecimal format: %v\nError: %v\n", input, err))
	}
	return bytes
}

func getByteSliceFromHex(prompt string) []byte {
	input := getString(prompt)
	bytes, err := hex.DecodeString(input)
	if err != nil {
		Exit(Fmt("Not in hexadecimal format: %v\nError: %v\n", input, err))
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
	stateDB := db_.GetDB("state")
	state := state_.LoadState(stateDB)

	// Get source pubkey
	srcPubKeyBytes := getByteSliceFromBase64("Enter source pubkey: ")
	r, n, err := bytes.NewReader(srcPubKeyBytes), new(int64), new(error)
	srcPubKey := account_.PubKeyDecoder(r, n, err).(account_.PubKey)
	if *err != nil {
		Exit(Fmt("Invalid PubKey. Error: %v", err))
	}

	// Get the state of the account.
	var srcAccount *account_.Account
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
	tx := &block_.SendTx{
		Inputs: []*block_.TxInput{
			&block_.TxInput{
				Address:   srcAddress,
				Amount:    srcSendAmount,
				Sequence:  srcSendSequence,
				Signature: account_.SignatureEd25519{},
				PubKey:    srcPubKey,
			},
		},
		Outputs: []*block_.TxOutput{
			&block_.TxOutput{
				Address: dstAddress,
				Amount:  dstSendAmount,
			},
		},
	}

	// Show the intermediate form.
	fmt.Printf("Generated tx: %X\n", binary.BinaryBytes(tx))

	// Get source privkey (for signing)
	srcPrivKeyBytes := getByteSliceFromBase64("Enter source privkey (for signing): ")
	r, n, err = bytes.NewReader(srcPrivKeyBytes), new(int64), new(error)
	srcPrivKey := account_.PrivKeyDecoder(r, n, err).(account_.PrivKey)
	if *err != nil {
		Exit(Fmt("Invalid PrivKey. Error: %v", err))
	}

	// Sign
	tx.Inputs[0].Signature = srcPrivKey.Sign(binary.BinaryBytes(tx))
	fmt.Printf("Signed tx: %X\n", binary.BinaryBytes(tx))
}
