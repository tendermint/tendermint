package main

import (
	"fmt"

	"github.com/tendermint/tendermint2/account"
	"github.com/tendermint/tendermint2/binary"
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
