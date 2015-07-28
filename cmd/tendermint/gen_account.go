package main

import (
	"fmt"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/wire"
)

func gen_account() {
	privAccount := acm.GenPrivAccount()
	privAccountJSONBytes := wire.JSONBytes(privAccount)
	fmt.Printf(`Generated a new account!

%v

`,
		string(privAccountJSONBytes),
	)
}
