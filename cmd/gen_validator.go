package main

import (
	"fmt"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"
)

func gen_validator() {

	privValidator := state.GenPrivValidator()
	privValidatorJSONBytes := privValidator.JSONBytes()
	fmt.Printf(`Generated a new validator!
Paste the following JSON into your %v file

%v

`,
		config.PrivValidatorFile(),
		string(privValidatorJSONBytes),
	)
}
