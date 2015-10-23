package main

import (
	"encoding/hex"
	"fmt"

	. "github.com/tendermint/go-common"
	cclient "github.com/tendermint/tendermint/rpc/core_client"
)

func get_account() {
	addrHex, err := Prompt("Enter the address of the account in HEX (e.g. 9FCBA7F840A0BFEBBE755E853C9947270A912D04):\n> ", "")
	if err != nil {
		Exit(Fmt("Error: %v", err))
	}
	cli := cclient.NewClient("http://localhost:46657", "JSONRPC")
	address, err := hex.DecodeString(addrHex)
	if err != nil {
		Exit(Fmt("Address was not hex: %v", addrHex))
	}
	res, err := cli.GetAccount(address)
	if res == nil {
		Exit(Fmt("Account does not exist: %X", address))
	}
	if err != nil {
		Exit(Fmt("Error fetching account: %v", err))
	}
	acc := res.Account

	fmt.Printf(`
Address:     %X
PubKey:      %v
Sequence:    %v
Balance:     %v
Permissions: %v
`,
		acc.Address,
		acc.PubKey,
		acc.Sequence,
		acc.Balance,
		acc.Permissions)
}
