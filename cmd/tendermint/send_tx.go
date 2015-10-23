package main

import (
	"encoding/hex"
	"fmt"
	"strconv"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/go-common"
	cclient "github.com/tendermint/tendermint/rpc/core_client"
	"github.com/tendermint/tendermint/types"
)

func getString(prompt string) string {
	input, err := Prompt(prompt, "")
	if err != nil {
		Exit(Fmt("Error reading input: %v", err))
	}
	return input
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

func send_tx() {

	// Get PrivAccount
	var privAccount *acm.PrivAccount
	secret := getString("Enter your secret, or just hit <Enter> to enter a private key in HEX.\n> ")
	if secret == "" {
		privKeyBytes := getByteSliceFromHex("Enter your private key in HEX (e.g. E353CAD81134A301A542AEBE2D2E4EF1A64A145117EC72743AE9C9D171A4AA69F3A7DD670A9E9307AAED000D97D5B3C07D90276BFCEEDA5ED11DA089A4E87A81):\n> ")
		privAccount = acm.GenPrivAccountFromPrivKeyBytes(privKeyBytes)
	} else {
		// Auto-detect private key hex
		if len(secret) == 128 {
			privKeyBytes, err := hex.DecodeString(secret)
			if err == nil {
				fmt.Println("Detected priv-key bytes...")
				privAccount = acm.GenPrivAccountFromPrivKeyBytes(privKeyBytes)
			} else {
				fmt.Println("That's a long seed...")
				privAccount = acm.GenPrivAccountFromSecret(secret)
			}
		} else {
			privAccount = acm.GenPrivAccountFromSecret(secret)
		}
	}
	pubKey := privAccount.PubKey

	// Get account data
	cli := cclient.NewClient("http://localhost:46657", "JSONRPC")
	res, err := cli.GetAccount(privAccount.Address)
	if err != nil {
		Exit(Fmt("Error fetching account: %v", err))
	}
	if res == nil {
		Exit(Fmt("No account was found with that secret/private-key"))
	}
	inputAcc := res.Account
	fmt.Printf(`
Source account:
Address:     %X
PubKey:      %v
Sequence:    %v
Balance:     %v
Permissions: %v
`,
		inputAcc.Address,
		pubKey,
		inputAcc.Sequence,
		inputAcc.Balance,
		inputAcc.Permissions)

	output := getByteSliceFromHex("\nEnter the output address in HEX:\n> ")
	amount := getInt64("Enter the amount to send:\n> ")

	// Construct transaction
	tx := types.NewSendTx()
	tx.AddInputWithNonce(pubKey, amount, inputAcc.Sequence+1)
	tx.AddOutput(output, amount)
	tx.Inputs[0].Signature = privAccount.Sign(config.GetString("chain_id"), tx)
	fmt.Println("Signed SendTx!: ", tx)

	// Sign up for events
	wsCli := cclient.NewWSClient("ws://localhost:46657/websocket")
	wsCli.Start()
	err = wsCli.Subscribe(types.EventStringAccInput(inputAcc.Address))
	if err != nil {
		Exit(Fmt("Error subscribing to account send event: %v", err))
	}

	// Broadcast transaction
	_, err = cli.BroadcastTx(tx)
	if err != nil {
		Exit(Fmt("Error broadcasting transaction: %v", err))
	}
	fmt.Println("Waiting for confirmation...")

	_ = <-wsCli.EventsCh
	fmt.Println("Confirmed.")
}
