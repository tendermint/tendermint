// +build gen_account

package main

import (
	"encoding/base64"
	"fmt"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"
)

func main() {

	// Parse config flags
	config.ParseFlags()

	// Generate private account
	privAccount := state.GenPrivAccount()

	fmt.Printf(`Generated account:
Account Public Key:  %X
            (base64) %v
Account Private Key: %X
            (base64) %v
`,
		privAccount.PubKey,
		base64.StdEncoding.EncodeToString(privAccount.PubKey),
		privAccount.PrivKey,
		base64.StdEncoding.EncodeToString(privAccount.PrivKey))
}
