package main

import (
	"fmt"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
)

func gen_account() {
	privAccount := account.GenPrivAccount()
	privAccountJSONBytes := binary.JSONBytes(privAccount)
	fmt.Printf(`Generated a new account!

%v

`,
		string(privAccountJSONBytes),
	)
}
