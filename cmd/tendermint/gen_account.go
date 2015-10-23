
package main

import (
	"fmt"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
)

func gen_account() {

	secret, err := Prompt(`Enter your desired secret, or just hit <Enter> to generate a random account.
IMPORTANT: If you don't know what a dictionary attack is, just hit Enter
> `, "")
	if err != nil {
		Exit(Fmt("Not sure what happened: %v", err))
	}

	if secret == "" {
		privAccount := acm.GenPrivAccount()
		privAccountJSONBytes := wire.JSONBytes(privAccount)
		fmt.Printf(`Generated a new random account!

%v

`,
			string(privAccountJSONBytes),
		)
	} else {
		privAccount := acm.GenPrivAccountFromSecret(secret)
		privAccountJSONBytes := wire.JSONBytes(privAccount)
		fmt.Printf(`Generated a new account from secret: [%v]!

%v

`,
			secret,
			string(privAccountJSONBytes),
		)
	}
}
