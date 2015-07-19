package main

import (
	"fmt"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
)

func gen_account() {
	privAccount := acm.GenPrivAccount()
	privAccountJSONBytes := binary.JSONBytes(privAccount)
	fmt.Printf(`Generated a new account!

%v

`,
		string(privAccountJSONBytes),
	)
}
