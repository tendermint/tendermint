package main

import (
	"encoding/hex"
	"fmt"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
)

func gen_account() {

	// TODO: uh, write better logic.
	// Generate private account
	privAccount := GenPrivAccount()

	fmt.Printf(`Generated account:
Account Public Key:  %v
Account Private Key: %v
`,
		hex.EncodeToString(BinaryBytes(privAccount.PubKey)),
		hex.EncodeToString(BinaryBytes(privAccount.PrivKey)),
	)
}
