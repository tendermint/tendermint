package main

import (
	"encoding/base64"
	"fmt"

	. "github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/wallet"
)

func gen_account() {

	// TODO: uh, write better logic.
	// Generate private account
	privAccount := wallet.GenPrivAccount()

	fmt.Printf(`Generated account:
Account Public Key:  %X
            (base64) %v
Account Private Key: %X
            (base64) %v
`,
		privAccount.PubKey,
		base64.StdEncoding.EncodeToString(BinaryBytes(privAccount.PubKey)),
		privAccount.PrivKey,
		base64.StdEncoding.EncodeToString(BinaryBytes(privAccount.PrivKey)))
}
