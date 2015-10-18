package main

import (
	"fmt"

	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
)

func show_validator() {
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	fmt.Println(string(wire.JSONBytes(privValidator.PubKey)))
}
