package main

import (
	"fmt"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

func gen_validator() {

	privValidator := types.GenPrivValidator()
	privValidatorJSONBytes := wire.JSONBytesPretty(privValidator)
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.GetString("priv_validator_file"),
		string(privValidatorJSONBytes),
	)
}
