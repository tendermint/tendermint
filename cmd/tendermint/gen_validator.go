package main

import (
	"fmt"

	"github.com/eris-ltd/tendermint/types"
	"github.com/eris-ltd/tendermint/wire"
)

func gen_validator() {

	privValidator := types.GenPrivValidator()
	privValidatorJSONBytes := wire.JSONBytes(privValidator)
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.GetString("priv_validator_file"),
		string(privValidatorJSONBytes),
	)
}
